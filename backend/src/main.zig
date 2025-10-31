const std = @import("std");
const net = std.net;
const print = std.debug.print;
const crypto = std.crypto;
const base64 = std.base64;
const pg = @import("pg");
const zlay_db = @import("zlay-db");
const db_config = @import("db_config.zig");
const auth = @import("auth.zig");

const Request = struct {
    method: []const u8,
    path: []const u8,
    headers: std.StringHashMap([]const u8),
    body: []const u8,
};

const Response = struct {
    status_code: u16,
    headers: std.StringHashMap([]const u8),
    body: []const u8,

    fn init(allocator: std.mem.Allocator) Response {
        return Response{
            .status_code = 200,
            .headers = std.StringHashMap([]const u8).init(allocator),
            .body = "",
        };
    }

    fn json(self: *Response, allocator: std.mem.Allocator, data: []const u8) void {
        self.headers.put("Content-Type", "application/json") catch {};
        // Add security headers
        self.headers.put("X-Content-Type-Options", "nosniff") catch {};
        self.headers.put("X-Frame-Options", "DENY") catch {};
        self.headers.put("X-XSS-Protection", "1; mode=block") catch {};
        self.headers.put("Referrer-Policy", "strict-origin-when-cross-origin") catch {};
        self.body = allocator.dupe(u8, data) catch "";
    }

    fn text(self: *Response, allocator: std.mem.Allocator, data: []const u8) void {
        self.headers.put("Content-Type", "text/plain") catch {};
        self.body = allocator.dupe(u8, data) catch "";
    }

    fn toString(self: *Response, allocator: std.mem.Allocator) ![]const u8 {
        var status_text: []const u8 = "";
        switch (self.status_code) {
            200 => status_text = "OK",
            404 => status_text = "Not Found",
            500 => status_text = "Internal Server Error",
            else => status_text = "Unknown",
        }

        var buffer = std.ArrayList(u8).initCapacity(allocator, 256) catch unreachable;
        defer buffer.deinit(allocator);

        try buffer.writer(allocator).print("HTTP/1.1 {d} {s}\r\n", .{ self.status_code, status_text });

        var it = self.headers.iterator();
        while (it.next()) |entry| {
            try buffer.writer(allocator).print("{s}: {s}\r\n", .{ entry.key_ptr.*, entry.value_ptr.* });
        }

        try buffer.writer(allocator).print("Content-Length: {d}\r\n\r\n", .{self.body.len});
        try buffer.appendSlice(allocator, self.body);

        return buffer.toOwnedSlice(allocator);
    }
};

// Simple rate limiter for API endpoints
const RateLimiter = struct {
    const RequestRecord = struct {
        timestamp: i64,
        count: u32,
    };

    var requests: std.StringHashMap(RequestRecord) = undefined;
    var mutex: std.Thread.Mutex = undefined;

    fn init() void {
        requests = std.StringHashMap(RequestRecord).init(std.heap.page_allocator);
        mutex = std.Thread.Mutex{};
    }

    fn deinit() void {
        requests.deinit();
    }

    fn isAllowed(client_id: []const u8, max_requests: u32, window_seconds: i64) bool {
        mutex.lock();
        defer mutex.unlock();

        const current_time = std.time.timestamp();

        if (requests.get(client_id)) |record| {
            const time_diff = current_time - record.timestamp;

            // Reset window if expired
            if (time_diff >= window_seconds) {
                const new_record = RequestRecord{ .timestamp = current_time, .count = 1 };
                requests.put(client_id, new_record) catch {};
                return true;
            }

            // Check if under limit
            if (record.count < max_requests) {
                const updated_record = RequestRecord{ .timestamp = record.timestamp, .count = record.count + 1 };
                requests.put(client_id, updated_record) catch {};
                return true;
            }

            return false; // Rate limit exceeded
        } else {
            // First request from this client
            const record = RequestRecord{ .timestamp = current_time, .count = 1 };
            requests.put(client_id, record) catch {};
            return true;
        }
    }
};

fn extractDomainFromOrigin(origin: []const u8) []const u8 {
    // Remove protocol (http:// or https://)
    const without_protocol = if (std.mem.startsWith(u8, origin, "https://")) origin["https://".len..] else if (std.mem.startsWith(u8, origin, "http://")) origin["http://".len..] else origin;

    // Remove port if present (everything after first colon)
    const colon_pos = std.mem.indexOfScalar(u8, without_protocol, ':');
    const domain = if (colon_pos) |pos| without_protocol[0..pos] else without_protocol;

    return domain;
}

fn setCorsHeaders(res: *Response, req: *Request) void {
    // Check for original origin from proxy (development) or use direct origin
    const origin = req.headers.get("X-Original-Origin") orelse
        req.headers.get("Origin") orelse
        req.headers.get("origin") orelse
        "http://localhost:5173";

    res.headers.put("Access-Control-Allow-Origin", origin) catch {};
    res.headers.put("Access-Control-Allow-Credentials", "true") catch {};
    res.headers.put("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS") catch {};
    res.headers.put("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Original-Origin") catch {};
}

const Router = struct {
    const HandlerFn = *const fn (*Request, *Response, std.mem.Allocator, *zlay_db.Database) void;
    const SimpleHandlerFn = *const fn (*Request, *Response, std.mem.Allocator) void;
    const WebSocketHandlerFn = *const fn (*WebSocketConnection, std.mem.Allocator) void;
    routes: std.StringHashMap(HandlerFn),
    simple_routes: std.StringHashMap(SimpleHandlerFn),
    ws_routes: std.StringHashMap(WebSocketHandlerFn),

    fn init(allocator: std.mem.Allocator) Router {
        return Router{
            .routes = std.StringHashMap(HandlerFn).init(allocator),
            .simple_routes = std.StringHashMap(SimpleHandlerFn).init(allocator),
            .ws_routes = std.StringHashMap(WebSocketHandlerFn).init(allocator),
        };
    }

    fn get(self: *Router, allocator: std.mem.Allocator, path: []const u8, handler: HandlerFn) void {
        var route = std.ArrayList(u8).initCapacity(allocator, 64) catch unreachable;
        defer route.deinit(allocator);
        route.writer(allocator).print("GET {s}", .{path}) catch {};
        self.routes.put(route.toOwnedSlice(allocator) catch "", handler) catch {};
    }

    fn getSimple(self: *Router, allocator: std.mem.Allocator, path: []const u8, handler: SimpleHandlerFn) void {
        var route = std.ArrayList(u8).initCapacity(allocator, 64) catch unreachable;
        defer route.deinit(allocator);
        route.writer(allocator).print("GET {s}", .{path}) catch {};
        self.simple_routes.put(route.toOwnedSlice(allocator) catch "", handler) catch {};
    }

    fn post(self: *Router, allocator: std.mem.Allocator, path: []const u8, handler: HandlerFn) void {
        var route = std.ArrayList(u8).initCapacity(allocator, 64) catch unreachable;
        defer route.deinit(allocator);
        route.writer(allocator).print("POST {s}", .{path}) catch {};
        const route_str = route.toOwnedSlice(allocator) catch "";
        std.debug.print("Registering POST route: {s}\n", .{route_str});
        self.routes.put(route_str, handler) catch {};
    }

    fn options(self: *Router, allocator: std.mem.Allocator, path: []const u8, handler: HandlerFn) void {
        var route = std.ArrayList(u8).initCapacity(allocator, 64) catch unreachable;
        defer route.deinit(allocator);
        route.writer(allocator).print("OPTIONS {s}", .{path}) catch {};
        self.routes.put(route.toOwnedSlice(allocator) catch "", handler) catch {};
    }

    fn put(self: *Router, allocator: std.mem.Allocator, path: []const u8, handler: HandlerFn) void {
        var route = std.ArrayList(u8).initCapacity(allocator, 64) catch unreachable;
        defer route.deinit(allocator);
        route.writer(allocator).print("PUT {s}", .{path}) catch {};
        self.routes.put(route.toOwnedSlice(allocator) catch "", handler) catch {};
    }

    fn delete(self: *Router, allocator: std.mem.Allocator, path: []const u8, handler: HandlerFn) void {
        var route = std.ArrayList(u8).initCapacity(allocator, 64) catch unreachable;
        defer route.deinit(allocator);
        route.writer(allocator).print("DELETE {s}", .{path}) catch {};
        self.routes.put(route.toOwnedSlice(allocator) catch "", handler) catch {};
    }

    fn websocket(self: *Router, allocator: std.mem.Allocator, path: []const u8, handler: WebSocketHandlerFn) void {
        var route = std.ArrayList(u8).initCapacity(allocator, 64) catch unreachable;
        defer route.deinit(allocator);
        route.writer(allocator).print("GET {s}", .{path}) catch {};
        self.ws_routes.put(route.toOwnedSlice(allocator) catch "", handler) catch {};
    }

    fn handle(self: *Router, method: []const u8, path: []const u8, req: *const Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) bool {

        // Try exact match first for auth routes (with pool)
        var exact_route = std.ArrayList(u8).initCapacity(allocator, 64) catch unreachable;
        defer exact_route.deinit(allocator);
        exact_route.writer(allocator).print("{s} {s}", .{ method, path }) catch {};

        if (self.routes.get(exact_route.items)) |handler| {

            // Rate limiting disabled for development
            // if (std.mem.startsWith(u8, path, "/api/auth/")) {
            //     const client_ip = req.headers.get("x-forwarded-for") orelse "127.0.0.1";
            //     if (!RateLimiter.isAllowed(client_ip, 10, 60)) { // 10 requests per minute
            //         res.status_code = 429;
            //         res.headers.put("Retry-After", "60") catch {};
            //         const error_json = "{\"error\": \"Rate limit exceeded. Please try again later.\"}";
            //         res.json(allocator, error_json);
            //         return true;
            //     }
            // }

            handler(@constCast(req), res, allocator, db);
            return true;
        }

        // Try simple routes (without pool)
        if (self.simple_routes.get(exact_route.items)) |handler| {
            handler(@constCast(req), res, allocator);
            return true;
        }

        // Try wildcard matches
        var it = self.routes.iterator();
        while (it.next()) |entry| {
            const route_key = entry.key_ptr.*;
            if (std.mem.indexOfScalar(u8, route_key, ' ')) |space_pos| {
                const route_method = route_key[0..space_pos];
                const route_pattern = route_key[space_pos + 1 ..];

                if (std.mem.eql(u8, method, route_method) and matchesPattern(path, route_pattern)) {
                    const handler = entry.value_ptr.*;
                    handler(@constCast(req), res, allocator, db);
                    return true;
                }
            }
        }

        return false;
    }

    fn matchesPattern(path: []const u8, pattern: []const u8) bool {
        var path_parts = std.mem.splitScalar(u8, path, '/');
        var pattern_parts = std.mem.splitScalar(u8, pattern, '/');

        while (true) {
            const path_part = path_parts.next();
            const pattern_part = pattern_parts.next();

            if (pattern_part == null) {
                // Pattern ended, path must also end
                return path_part == null;
            }

            if (path_part == null) {
                // Path ended, but pattern not, unless pattern is *
                if (std.mem.eql(u8, pattern_part.?, "*")) {
                    // * can match empty
                    continue;
                }
                return false;
            }

            if (std.mem.eql(u8, pattern_part.?, "*")) {
                // * matches anything, continue
                continue;
            }

            if (!std.mem.eql(u8, path_part.?, pattern_part.?)) {
                return false;
            }
        }
    }

    fn handleWebSocket(self: *Router, path: []const u8, connection: *WebSocketConnection, allocator: std.mem.Allocator) bool {
        var route = std.ArrayList(u8).initCapacity(allocator, 64) catch unreachable;
        defer route.deinit(allocator);
        route.writer(allocator).print("GET {s}", .{path}) catch {};

        if (self.ws_routes.get(route.items)) |handler| {
            handler(connection, allocator);
            return true;
        }
        return false;
    }
};

fn parseRequest(buffer: []const u8, allocator: std.mem.Allocator) !Request {
    var lines = std.mem.tokenizeScalar(u8, buffer, '\n');

    const request_line = lines.next() orelse return error.InvalidRequest;
    var parts = std.mem.tokenizeScalar(u8, request_line, ' ');

    const method = parts.next() orelse return error.InvalidRequest;
    const path = parts.next() orelse return error.InvalidRequest;

    var headers = std.StringHashMap([]const u8).init(allocator);

    while (lines.next()) |line| {
        if (line.len == 0) break;

        if (std.mem.indexOfScalar(u8, line, ':')) |colon_idx| {
            const key = std.mem.trim(u8, line[0..colon_idx], " \r");
            const value = std.mem.trim(u8, line[colon_idx + 1 ..], " \r");
            headers.put(key, value) catch {};
        }
    }

    var body_start: usize = 0;
    if (std.mem.indexOf(u8, buffer, "\r\n\r\n")) |double_crlf| {
        body_start = double_crlf + 4;
    }

    const body = buffer[body_start..];

    return Request{
        .method = allocator.dupe(u8, method) catch "",
        .path = allocator.dupe(u8, path) catch "",
        .headers = headers,
        .body = allocator.dupe(u8, body) catch "",
    };
}

fn isWebSocketUpgrade(req: *const Request) bool {
    const connection = req.headers.get("Connection") orelse return false;
    const upgrade = req.headers.get("Upgrade") orelse return false;
    const key = req.headers.get("Sec-WebSocket-Key") orelse return false;
    const version = req.headers.get("Sec-WebSocket-Version") orelse return false;

    return std.mem.indexOf(u8, connection, "Upgrade") != null and
        std.mem.eql(u8, upgrade, "websocket") and
        key.len > 0 and
        std.mem.eql(u8, version, "13");
}

fn generateWebSocketAcceptKey(websocket_key: []const u8, allocator: std.mem.Allocator) ![]const u8 {
    const magic_string = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11";
    var concat = std.ArrayList(u8).initCapacity(allocator, 128) catch unreachable;
    defer concat.deinit(allocator);

    try concat.appendSlice(allocator, websocket_key);
    try concat.appendSlice(allocator, magic_string);

    var hash: [20]u8 = undefined;
    crypto.hash.Sha1.hash(concat.items, &hash, .{});

    const encoded = try allocator.alloc(u8, base64.standard.Encoder.calcSize(hash.len));
    _ = base64.standard.Encoder.encode(encoded, &hash);
    return encoded;
}

fn homeHandler(req: *Request, res: *Response, allocator: std.mem.Allocator) void {
    _ = req;
    const json = "{\"message\": \"Hello, World!\", \"status\": \"ok\"}";
    res.json(allocator, json);
}

fn apiHandler(req: *Request, res: *Response, allocator: std.mem.Allocator) void {
    _ = req;
    const json = "{\"api\": \"v1\", \"endpoints\": [\"/\", \"/api\", \"/ws\"]}";
    res.json(allocator, json);
}

fn serveStaticFile(req: *Request, res: *Response, allocator: std.mem.Allocator, file_path: []const u8) void {
    _ = req;

    const full_path = std.fs.path.join(allocator, &[_][]const u8{ "../frontend/dist", file_path }) catch {
        res.status_code = 500;
        const error_json = "{\"error\": \"Internal server error\"}";
        res.json(allocator, error_json);
        return;
    };
    defer allocator.free(full_path);

    const file = std.fs.cwd().openFile(full_path, .{}) catch {
        res.status_code = 404;
        const not_found = "{\"error\": \"File not found\"}";
        res.json(allocator, not_found);
        return;
    };
    defer file.close();

    const content = file.readToEndAlloc(allocator, 2 * 1024 * 1024) catch {
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to read file\"}";
        res.json(allocator, error_json);
        return;
    };
    defer allocator.free(content);

    // Set appropriate content type based on file extension
    if (std.mem.endsWith(u8, file_path, ".html")) {
        res.headers.put("Content-Type", "text/html") catch {};
    } else if (std.mem.endsWith(u8, file_path, ".css")) {
        res.headers.put("Content-Type", "text/css") catch {};
    } else if (std.mem.endsWith(u8, file_path, ".js")) {
        res.headers.put("Content-Type", "application/javascript") catch {};
    } else if (std.mem.endsWith(u8, file_path, ".ico")) {
        res.headers.put("Content-Type", "image/x-icon") catch {};
    } else {
        res.headers.put("Content-Type", "application/octet-stream") catch {};
    }

    res.body = content;
}

fn spaHandler(req: *Request, res: *Response, allocator: std.mem.Allocator) void {
    serveStaticFile(req, res, allocator, "index.html");
}

fn healthHandler(req: *Request, res: *Response, allocator: std.mem.Allocator) void {
    _ = req;
    const current_time = std.time.timestamp();
    const response_json = std.fmt.allocPrint(allocator,
        \\{{"status": "healthy", "timestamp": "{d}", "version": "1.0.0"}}
    , .{current_time}) catch {
        res.status_code = 500;
        const error_json = "{\"error\": \"Internal server error\"}";
        res.json(allocator, error_json);
        return;
    };
    defer allocator.free(response_json);

    res.status_code = 200;
    res.json(allocator, response_json);
}

// Helper function to extract client_id from request
fn getClientId(req: *Request, allocator: std.mem.Allocator, db: *zlay_db.Database) ?[]const u8 {
    // Try to get client_id from X-Client-ID header first
    if (req.headers.get("X-Client-ID")) |client_id_header| {
        return allocator.dupe(u8, client_id_header) catch null;
    }

    // Try to get client from Origin/Referer headers first (for proper domain isolation)
    var resolved_domain: ?[]const u8 = null;
    var needs_free = false;

    // Priority: Origin -> Referer -> Host
    if (req.headers.get("Origin")) |origin| {
        resolved_domain = extractDomainFromOrigin(origin);
        needs_free = false; // extractDomainFromOrigin returns a slice, no free needed
    } else if (req.headers.get("Referer")) |referer| {
        resolved_domain = extractDomainFromOrigin(referer);
        needs_free = false; // extractDomainFromOrigin returns a slice, no free needed
    } else if (req.headers.get("Host")) |host| {
        // Fallback to Host header (remove port)
        resolved_domain = if (std.mem.indexOfScalar(u8, host, ':')) |colon_pos|
            allocator.dupe(u8, host[0..colon_pos]) catch null
        else
            allocator.dupe(u8, host) catch null;
        needs_free = true; // allocator.dupe needs to be freed
    }

    if (resolved_domain) |domain| {
        if (needs_free) {
            defer allocator.free(domain);
        }

        // Look up client by domain (try exact match first)
        const domain_result = db.query("SELECT client_id::text FROM domains WHERE domain = $1 AND is_active = true LIMIT 1", .{domain}) catch null;
        if (domain_result) |result| {
            defer result.deinit();
            if (result.rows.len > 0) {
                return allocator.dupe(u8, result.rows[0].values[0].text) catch null;
            }
        }

        // If no exact match, try normalizing the stored domains (remove protocol/port)
        const normalized_domain_result = db.query("SELECT client_id::text, domain FROM domains WHERE is_active = true", .{}) catch null;
        if (normalized_domain_result) |result| {
            defer result.deinit();
            for (result.rows) |row| {
                const stored_domain = row.values[1].text;
                // Normalize stored domain (remove protocol and port)
                const normalized_stored = extractDomainFromOrigin(stored_domain);
                if (std.mem.eql(u8, domain, normalized_stored)) {
                    return allocator.dupe(u8, row.values[0].text) catch null;
                }
            }
        }
    }

    // No valid client found - return null to trigger client validation error
    return null;
}

fn loginHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Parse JSON body first to check for root user
    const json_str = req.body;

    // Quick check if this might be root user login
    const is_root_attempt = std.mem.indexOf(u8, json_str, "\"username\":\"root\"") != null;

    // Get client ID
    var client_id_opt = getClientId(req, allocator, db);

    // If no client found but this might be root, use default client
    if (client_id_opt == null and is_root_attempt) {
        // For root user, find the default client
        const default_client_result = db.query("SELECT id::text FROM clients WHERE is_active = true ORDER BY created_at ASC LIMIT 1", .{}) catch {
            res.status_code = 400;
            const error_json = "{\"error\": \"Invalid client\"}";
            res.json(allocator, error_json);
            return;
        };
        defer default_client_result.deinit();

        if (default_client_result.rows.len > 0) {
            client_id_opt = allocator.dupe(u8, default_client_result.rows[0].values[0].text) catch {
                res.status_code = 500;
                const error_json = "{\"error\": \"Internal server error\"}";
                res.json(allocator, error_json);
                return;
            };
        } else {
            res.status_code = 400;
            const error_json = "{\"error\": \"Invalid client\"}";
            res.json(allocator, error_json);
            return;
        }
    } else if (client_id_opt == null) {
        // Not root and no valid client
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid client\"}";
        res.json(allocator, error_json);
        return;
    }

    const client_id = client_id_opt.?; // Now we know it's not null
    defer allocator.free(client_id);

    // Continue with JSON parsing using the already declared json_str

    // Manual JSON parsing to ensure correctness
    const username_key = "\"username\":\"";
    const password_key = "\"password\":\"";

    const username_start = std.mem.indexOf(u8, json_str, username_key) orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid JSON format\"}";
        res.json(allocator, error_json);
        return;
    };
    const username_quote_start = username_start + username_key.len;
    const username_end = std.mem.indexOf(u8, json_str[username_quote_start..], "\"") orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid JSON format\"}";
        res.json(allocator, error_json);
        return;
    };
    const username = json_str[username_quote_start .. username_quote_start + username_end];

    const password_start = std.mem.indexOf(u8, json_str, password_key) orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid JSON format\"}";
        res.json(allocator, error_json);
        return;
    };
    const password_quote_start = password_start + password_key.len;
    const password_end = std.mem.indexOf(u8, json_str[password_quote_start..], "\"") orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid JSON format\"}";
        res.json(allocator, error_json);
        return;
    };
    const password = json_str[password_quote_start .. password_quote_start + password_end];

    // Validate input
    if (username.len < 3 or password.len < 8) {
        res.status_code = 400;
        const error_json = "{\"error\": \"Username must be at least 3 characters and password must be at least 8 characters\"}";
        res.json(allocator, error_json);
        return;
    }

    // Login user
    const login_req = auth.LoginRequest{
        .client_id = client_id,
        .username = username,
        .password = password,
    };

    const result = auth.loginUser(allocator, db, login_req) catch |err| {
        std.log.err("Login failed with error: {}", .{err});
        switch (err) {
            error.UserNotFound => {
                std.log.info("Login failed: User '{s}' not found for client '{s}'", .{ username, client_id });
                res.status_code = 401;
                const error_json = "{\"error\": \"Invalid username or password\"}";
                res.json(allocator, error_json);
                return;
            },
            error.InvalidPassword => {
                std.log.info("Login failed: Invalid password for user '{s}'", .{username});
                res.status_code = 401;
                const error_json = "{\"error\": \"Invalid username or password\"}";
                res.json(allocator, error_json);
                return;
            },
            error.InvalidClient => {
                std.log.info("Login failed: Invalid client '{s}'", .{client_id});
                res.status_code = 400;
                const error_json = "{\"error\": \"Invalid client\"}";
                res.json(allocator, error_json);
                return;
            },
            error.DatabaseError => {
                std.log.err("Login failed: Database error for user '{s}'", .{username});
                res.status_code = 500;
                const error_json = "{\"error\": \"Database error\"}";
                res.json(allocator, error_json);
                return;
            },
            else => {
                std.log.err("Login failed with unexpected error: {}", .{err});
                res.status_code = 500;
                const error_json = "{\"error\": \"Login failed\"}";
                res.json(allocator, error_json);
                return;
            },
        }
    };
    defer {
        allocator.free(result.user.id);
        allocator.free(result.user.username);
        allocator.free(result.user.password_hash);
        allocator.free(result.user.created_at);
        allocator.free(result.token);
    }

    // Set session cookie with dynamic domain
    const origin = req.headers.get("X-Original-Origin") orelse
        req.headers.get("Origin") orelse
        req.headers.get("origin") orelse
        "http://localhost:5173";
    const cookie_domain = extractDomainFromOrigin(origin);
    const cookie_value = std.fmt.allocPrint(allocator, "session_token={s}; Domain={s}; Path=/; Max-Age=86400; HttpOnly; SameSite=Lax", .{ result.token, cookie_domain }) catch {
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create cookie\"}";
        res.json(allocator, error_json);
        return;
    };

    res.headers.put("Set-Cookie", cookie_value) catch {};

    // Return success response
    const response_json = std.fmt.allocPrint(allocator,
        \\{{"success": true, "message": "Login successful", "user": {{ "id": "{s}", "username": "{s}" }}}}
    , .{ result.user.id, result.user.username }) catch {
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create response\"}";
        res.json(allocator, error_json);
        return;
    };

    res.json(allocator, response_json);
}

fn registerHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get client ID
    const client_id = getClientId(req, allocator, db) orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid client\"}";
        res.json(allocator, error_json);
        return;
    };
    defer allocator.free(client_id);

    // Parse JSON body
    const json_str = req.body;
    var parsed = std.json.parseFromSlice(struct {
        username: []const u8,
        password: []const u8,
    }, allocator, json_str, .{}) catch {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid JSON format\"}";
        res.json(allocator, error_json);
        return;
    };
    defer parsed.deinit();

    const username = parsed.value.username;
    const password = parsed.value.password;

    // Validate input
    if (username.len < 3 or password.len < 8) {
        res.status_code = 400;
        const error_json = "{\"error\": \"Username must be at least 3 characters and password must be at least 8 characters\"}";
        res.json(allocator, error_json);
        return;
    }

    const register_req = auth.RegisterRequest{
        .client_id = client_id,
        .username = username,
        .password = password,
    };

    const user = auth.registerUser(allocator, db, register_req) catch |err| {
        std.log.err("Registration failed with error: {}", .{err});
        switch (err) {
            error.UserAlreadyExists => {
                std.log.info("Registration failed: User '{s}' already exists for client '{s}'", .{ username, client_id });
                res.status_code = 409;
                const error_json = "{\"error\": \"Username already exists\"}";
                res.json(allocator, error_json);
                return;
            },
            error.DatabaseError => {
                std.log.err("Registration failed: Database error for user '{s}'", .{username});
                res.status_code = 500;
                const error_json = "{\"error\": \"Database error\"}";
                res.json(allocator, error_json);
                return;
            },
            else => {
                std.log.err("Registration failed with unexpected error: {}", .{err});
                res.status_code = 500;
                const error_json = "{\"error\": \"Registration failed\"}";
                res.json(allocator, error_json);
                return;
            },
        }
    };
    defer {
        allocator.free(user.id);
        allocator.free(user.username);
        allocator.free(user.password_hash);
        allocator.free(user.created_at);
    }

    // Auto-login after registration
    const login_req = auth.LoginRequest{
        .client_id = client_id,
        .username = username,
        .password = password,
    };

    const result = auth.loginUser(allocator, db, login_req) catch |err| {
        std.log.err("Auto-login after registration failed with error: {}", .{err});
        switch (err) {
            error.UserNotFound, error.InvalidPassword => {
                std.log.err("Auto-login failed: Unexpected error for newly created user '{s}'", .{username});
                res.status_code = 500;
                const error_json = "{\"error\": \"Registration successful but login failed\"}";
                res.json(allocator, error_json);
                return;
            },
            error.DatabaseError => {
                std.log.err("Auto-login failed: Database error for user '{s}'", .{username});
                res.status_code = 500;
                const error_json = "{\"error\": \"Database error during login\"}";
                res.json(allocator, error_json);
                return;
            },
            else => {
                std.log.err("Auto-login failed with unexpected error: {}", .{err});
                res.status_code = 500;
                const error_json = "{\"error\": \"Login failed\"}";
                res.json(allocator, error_json);
                return;
            },
        }
    };
    defer {
        allocator.free(result.user.id);
        allocator.free(result.user.username);
        allocator.free(result.user.password_hash);
        allocator.free(result.user.created_at);
        allocator.free(result.token);
    }

    // Set session cookie with dynamic domain
    const origin = req.headers.get("X-Original-Origin") orelse
        req.headers.get("Origin") orelse
        req.headers.get("origin") orelse
        "http://localhost:5173";
    const cookie_domain = extractDomainFromOrigin(origin);
    const cookie_value = std.fmt.allocPrint(allocator, "session_token={s}; Domain={s}; Path=/; Max-Age=86400; HttpOnly; SameSite=Lax", .{ result.token, cookie_domain }) catch {
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create cookie\"}";
        res.json(allocator, error_json);
        return;
    };

    res.headers.put("Set-Cookie", cookie_value) catch {};

    // Return success response
    const response_json = std.fmt.allocPrint(allocator,
        \\{{"success": true, "message": "Login successful", "user": {{ "id": "{s}", "username": "{s}" }}}}
    , .{ result.user.id, result.user.username }) catch {
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create response\"}";
        res.json(allocator, error_json);
        return;
    };

    res.json(allocator, response_json);
}

fn logoutHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get session token from cookie
    const cookie_header = req.headers.get("Cookie") orelse {
        res.status_code = 401;
        const error_json = "{\"error\": \"No session found\"}";
        res.json(allocator, error_json);
        return;
    };

    var token: ?[]const u8 = null;
    var cookie_iter = std.mem.tokenizeScalar(u8, cookie_header, ';');
    while (cookie_iter.next()) |cookie| {
        const trimmed = std.mem.trim(u8, cookie, " ");
        if (std.mem.startsWith(u8, trimmed, "session_token=")) {
            token = trimmed["session_token=".len..];
            break;
        }
    }

    if (token) |session_token| {
        auth.logoutUser(allocator, db, session_token) catch {};
    }

    // Clear session cookie
    res.headers.put("Set-Cookie", "session_token=; HttpOnly; SameSite=Strict; Path=/; Max-Age=0") catch {};

    const success_json = "{\"success\": true, \"message\": \"Logout successful\"}";
    res.json(allocator, success_json);
}

fn corsHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    _ = db;
    // Add CORS headers for preflight requests
    setCorsHeaders(res, req);
    res.status_code = 200;
    res.json(allocator, "{}");
}

fn getProjectsHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, db) orelse return;

    // Get projects
    const projects = auth.database.getProjectsByUser(allocator, db, user.id) catch |err| {
        std.log.err("Failed to get projects: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to get projects\"}";
        res.json(allocator, error_json);
        return;
    };
    defer {
        for (projects) |p| {
            allocator.free(p.id);
            allocator.free(p.user_id);
            allocator.free(p.name);
            allocator.free(p.description);
            allocator.free(p.created_at);
        }
        allocator.free(projects);
    }

    // Build JSON response
    var json = std.ArrayList(u8).initCapacity(allocator, 1024) catch unreachable;
    defer json.deinit(allocator);
    json.writer(allocator).writeAll("[") catch {};

    for (projects, 0..) |p, i| {
        if (i > 0) json.writer(allocator).writeAll(",") catch {};
        json.writer(allocator).print(
            \\{{"id":"{s}","user_id":"{s}","name":"{s}","description":"{s}","is_active":{},"created_at":"{s}"}}
        , .{ p.id, p.user_id, p.name, p.description, p.is_active, p.created_at }) catch {};
    }
    json.writer(allocator).writeAll("]") catch {};

    res.json(allocator, json.items);
}

fn createProjectHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, db) orelse return;

    // Parse JSON body
    const json_str = req.body;
    var parsed = std.json.parseFromSlice(struct {
        name: []const u8,
        description: []const u8,
    }, allocator, json_str, .{}) catch {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid JSON format\"}";
        res.json(allocator, error_json);
        return;
    };
    defer parsed.deinit();

    const data = parsed.value;

    // Validate
    if (data.name.len == 0) {
        res.status_code = 400;
        const error_json = "{\"error\": \"Name is required\"}";
        res.json(allocator, error_json);
        return;
    }

    // Create project
    const project_id = auth.database.createProject(allocator, db, user.id, data.name, data.description) catch |err| {
        std.log.err("Failed to create project: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create project\"}";
        res.json(allocator, error_json);
        return;
    };
    defer allocator.free(project_id);

    // Return success response
    const response_json = std.fmt.allocPrint(allocator,
        \\{{"success": true, "message": "Project created successfully", "project_id": "{s}"}}
    , .{project_id}) catch {
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create response\"}";
        res.json(allocator, error_json);
        return;
    };

    res.status_code = 201;
    res.json(allocator, response_json);
}

fn getProjectHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, db) orelse return;

    // Parse project ID from path
    const path_parts = std.mem.splitScalar(u8, req.path, '/');
    var iter = path_parts;
    _ = iter.next(); // api
    _ = iter.next(); // projects
    const project_id = iter.next() orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid project ID\"}";
        res.json(allocator, error_json);
        return;
    };

    // Get project
    const project = auth.database.getProjectById(allocator, db, project_id) catch |err| {
        std.log.err("Failed to get project: {}", .{err});
        res.status_code = 404;
        const error_json = "{\"error\": \"Project not found\"}";
        res.json(allocator, error_json);
        return;
    };
    defer {
        allocator.free(project.id);
        allocator.free(project.user_id);
        allocator.free(project.name);
        allocator.free(project.description);
        allocator.free(project.created_at);
    }

    // Check ownership
    if (!std.mem.eql(u8, project.user_id, user.id)) {
        res.status_code = 403;
        const error_json = "{\"error\": \"Access denied\"}";
        res.json(allocator, error_json);
        return;
    }

    // const response_json = std.fmt.allocPrint(allocator,
    //     \\{{"success": true, "message": "Login successful", "user": {{ "id": "{s}", "username": "{s}" }}}}
    // , .{ result.user.id, result.user.username }) catch {
    //     res.status_code = 500;
    //     const error_json = "{\"error\": \"Failed to create response\"}";
    //     res.json(allocator, error_json);
    //     return;
    // };

    // res.json(allocator, response_json);
}

fn updateProjectHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, db) orelse return;

    // Parse project ID from path
    const path_parts = std.mem.splitScalar(u8, req.path, '/');
    var iter = path_parts;
    _ = iter.next(); // api
    _ = iter.next(); // projects
    const project_id = iter.next() orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid project ID\"}";
        res.json(allocator, error_json);
        return;
    };

    // Parse JSON body
    const json_str = req.body;
    var parsed = std.json.parseFromSlice(struct {
        name: []const u8,
        description: []const u8,
    }, allocator, json_str, .{}) catch {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid JSON format\"}";
        res.json(allocator, error_json);
        return;
    };
    defer parsed.deinit();

    const data = parsed.value;

    // Validate
    if (data.name.len == 0) {
        res.status_code = 400;
        const error_json = "{\"error\": \"Name is required\"}";
        res.json(allocator, error_json);
        return;
    }

    // Check ownership (get project first)
    const project = auth.database.getProjectById(allocator, db, project_id) catch {
        res.status_code = 404;
        const error_json = "{\"error\": \"Project not found\"}";
        res.json(allocator, error_json);
        return;
    };
    defer {
        allocator.free(project.id);
        allocator.free(project.user_id);
        allocator.free(project.name);
        allocator.free(project.description);
        allocator.free(project.created_at);
    }

    if (!std.mem.eql(u8, project.user_id, user.id)) {
        res.status_code = 403;
        const error_json = "{\"error\": \"Access denied\"}";
        res.json(allocator, error_json);
        return;
    }

    // Update project
    auth.database.updateProject(db, project_id, data.name, data.description) catch |err| {
        std.log.err("Failed to update project: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to update project\"}";
        res.json(allocator, error_json);
        return;
    };

    const response_json = "{\"success\": true, \"message\": \"Project updated\"}";
    res.json(allocator, response_json);
}

fn deleteProjectHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, db) orelse return;

    // Parse project ID from path
    const path_parts = std.mem.splitScalar(u8, req.path, '/');
    var iter = path_parts;
    _ = iter.next(); // api
    _ = iter.next(); // projects
    const project_id = iter.next() orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid project ID\"}";
        res.json(allocator, error_json);
        return;
    };

    // Check ownership
    const project = auth.database.getProjectById(allocator, db, project_id) catch {
        res.status_code = 404;
        const error_json = "{\"error\": \"Project not found\"}";
        res.json(allocator, error_json);
        return;
    };
    defer {
        allocator.free(project.id);
        allocator.free(project.user_id);
        allocator.free(project.name);
        allocator.free(project.description);
        allocator.free(project.created_at);
    }

    if (!std.mem.eql(u8, project.user_id, user.id)) {
        res.status_code = 403;
        const error_json = "{\"error\": \"Access denied\"}";
        res.json(allocator, error_json);
        return;
    }

    // Delete project
    auth.database.deleteProject(db, project_id) catch |err| {
        std.log.err("Failed to delete project: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to delete project\"}";
        res.json(allocator, error_json);
        return;
    };

    const response_json = "{\"success\": true, \"message\": \"Project deleted\"}";
    res.json(allocator, response_json);
}

fn getDatasourcesHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, db) orelse return;

    // Parse project ID from path
    const path_parts = std.mem.splitScalar(u8, req.path, '/');
    var iter = path_parts;
    _ = iter.next(); // api
    _ = iter.next(); // projects
    const project_id = iter.next() orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid project ID\"}";
        res.json(allocator, error_json);
        return;
    };
    _ = iter.next(); // datasources

    // Check project ownership
    const project = auth.database.getProjectById(allocator, db, project_id) catch {
        res.status_code = 404;
        const error_json = "{\"error\": \"Project not found\"}";
        res.json(allocator, error_json);
        return;
    };
    defer {
        allocator.free(project.id);
        allocator.free(project.user_id);
        allocator.free(project.name);
        allocator.free(project.description);
        allocator.free(project.created_at);
    }

    if (!std.mem.eql(u8, project.user_id, user.id)) {
        res.status_code = 403;
        const error_json = "{\"error\": \"Access denied\"}";
        res.json(allocator, error_json);
        return;
    }

    // Get datasources
    const datasources = auth.database.getDatasourcesByProject(allocator, db, project_id) catch |err| {
        std.log.err("Failed to get datasources: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to get datasources\"}";
        res.json(allocator, error_json);
        return;
    };
    defer {
        for (datasources) |ds| {
            allocator.free(ds.id);
            allocator.free(ds.project_id);
            allocator.free(ds.name);
            allocator.free(ds.type);
            allocator.free(ds.config);
            allocator.free(ds.created_at);
        }
        allocator.free(datasources);
    }

    // Build JSON response
    var json = std.ArrayList(u8).initCapacity(allocator, 1024) catch unreachable;
    defer json.deinit(allocator);
    json.writer(allocator).writeAll("[") catch {};

    for (datasources, 0..) |ds, i| {
        if (i > 0) json.writer(allocator).writeAll(",") catch {};
        json.writer(allocator).print(
            \\{{"id":"{s}","project_id":"{s}","name":"{s}","type":"{s}","config":{s},"is_active":{},"created_at":"{s}"}}
        , .{ ds.id, ds.project_id, ds.name, ds.type, ds.config, ds.is_active, ds.created_at }) catch {};
    }
    json.writer(allocator).writeAll("]") catch {};

    res.json(allocator, json.items);
}

fn createDatasourceHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, db) orelse return;

    // Parse project ID from path
    const path_parts = std.mem.splitScalar(u8, req.path, '/');
    var iter = path_parts;
    _ = iter.next(); // api
    _ = iter.next(); // projects
    const project_id = iter.next() orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid project ID\"}";
        res.json(allocator, error_json);
        return;
    };
    _ = iter.next(); // datasources

    // Check project ownership
    const project = auth.database.getProjectById(allocator, db, project_id) catch {
        res.status_code = 404;
        const error_json = "{\"error\": \"Project not found\"}";
        res.json(allocator, error_json);
        return;
    };
    defer {
        allocator.free(project.id);
        allocator.free(project.user_id);
        allocator.free(project.name);
        allocator.free(project.description);
        allocator.free(project.created_at);
    }

    if (!std.mem.eql(u8, project.user_id, user.id)) {
        res.status_code = 403;
        const error_json = "{\"error\": \"Access denied\"}";
        res.json(allocator, error_json);
        return;
    }

    // Parse JSON body
    const json_str = req.body;
    var parsed = std.json.parseFromSlice(struct {
        name: []const u8,
        type: []const u8,
        config: []const u8,
    }, allocator, json_str, .{}) catch {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid JSON format\"}";
        res.json(allocator, error_json);
        return;
    };
    defer parsed.deinit();

    const data = parsed.value;

    // Validate
    if (data.name.len == 0 or data.type.len == 0) {
        res.status_code = 400;
        const error_json = "{\"error\": \"Name and type are required\"}";
        res.json(allocator, error_json);
        return;
    }

    // Create datasource
    const datasource_id = auth.database.createDatasource(allocator, db, project_id, data.name, data.type, data.config) catch |err| {
        std.log.err("Failed to create datasource: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create datasource\"}";
        res.json(allocator, error_json);
        return;
    };
    defer allocator.free(datasource_id);

    // Return success response
    const response_json = std.fmt.allocPrint(allocator,
        \\{{"success": true, "message": "Datasource created", "id": "{s}"}}
    , .{datasource_id}) catch {
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create response\"}";
        res.json(allocator, error_json);
        return;
    };
    defer allocator.free(response_json);

    res.status_code = 201;
    res.json(allocator, response_json);
}

fn getDatasourceHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, db) orelse return;

    // Parse project ID and datasource ID from path
    const path_parts = std.mem.splitScalar(u8, req.path, '/');
    var iter = path_parts;
    _ = iter.next(); // api
    _ = iter.next(); // projects
    const project_id = iter.next() orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid project ID\"}";
        res.json(allocator, error_json);
        return;
    };
    _ = iter.next(); // datasources
    const datasource_id = iter.next() orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid datasource ID\"}";
        res.json(allocator, error_json);
        return;
    };

    // Check project ownership
    const project = auth.database.getProjectById(allocator, db, project_id) catch {
        res.status_code = 404;
        const error_json = "{\"error\": \"Project not found\"}";
        res.json(allocator, error_json);
        return;
    };
    defer {
        allocator.free(project.id);
        allocator.free(project.user_id);
        allocator.free(project.name);
        allocator.free(project.description);
        allocator.free(project.created_at);
    }

    if (!std.mem.eql(u8, project.user_id, user.id)) {
        res.status_code = 403;
        const error_json = "{\"error\": \"Access denied\"}";
        res.json(allocator, error_json);
        return;
    }

    // Get datasource
    const datasource = auth.database.getDatasourceById(allocator, db, datasource_id) catch |err| {
        std.log.err("Failed to get datasource: {}", .{err});
        res.status_code = 404;
        const error_json = "{\"error\": \"Datasource not found\"}";
        res.json(allocator, error_json);
        return;
    };
    defer {
        allocator.free(datasource.id);
        allocator.free(datasource.project_id);
        allocator.free(datasource.name);
        allocator.free(datasource.type);
        allocator.free(datasource.config);
        allocator.free(datasource.created_at);
    }

    // Check if belongs to project
    if (!std.mem.eql(u8, datasource.project_id, project_id)) {
        res.status_code = 404;
        const error_json = "{\"error\": \"Datasource not found\"}";
        res.json(allocator, error_json);
        return;
    }

    // const response_json = std.fmt.allocPrint(allocator,
    //     \\{{"success": true, "message": "Login successful", "user": {{ "id": "{s}", "username": "{s}" }}}}
    // , .{ result.user.id, result.user.username }) catch {
    //     res.status_code = 500;
    //     const error_json = "{\"error\": \"Failed to create response\"}";
    //     res.json(allocator, error_json);
    //     return;
    // };

    // res.json(allocator, response_json);
}

fn updateDatasourceHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, db) orelse return;

    // Parse project ID and datasource ID from path
    const path_parts = std.mem.splitScalar(u8, req.path, '/');
    var iter = path_parts;
    _ = iter.next(); // api
    _ = iter.next(); // projects
    const project_id = iter.next() orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid project ID\"}";
        res.json(allocator, error_json);
        return;
    };
    _ = iter.next(); // datasources
    const datasource_id = iter.next() orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid datasource ID\"}";
        res.json(allocator, error_json);
        return;
    };

    // Check project ownership
    const project = auth.database.getProjectById(allocator, db, project_id) catch {
        res.status_code = 404;
        const error_json = "{\"error\": \"Project not found\"}";
        res.json(allocator, error_json);
        return;
    };
    defer {
        allocator.free(project.id);
        allocator.free(project.user_id);
        allocator.free(project.name);
        allocator.free(project.description);
        allocator.free(project.created_at);
    }

    if (!std.mem.eql(u8, project.user_id, user.id)) {
        res.status_code = 403;
        const error_json = "{\"error\": \"Access denied\"}";
        res.json(allocator, error_json);
        return;
    }

    // Parse JSON body
    const json_str = req.body;
    var parsed = std.json.parseFromSlice(struct {
        name: []const u8,
        type: []const u8,
        config: []const u8,
    }, allocator, json_str, .{}) catch {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid JSON format\"}";
        res.json(allocator, error_json);
        return;
    };
    defer parsed.deinit();

    const data = parsed.value;

    // Validate
    if (data.name.len == 0 or data.type.len == 0) {
        res.status_code = 400;
        const error_json = "{\"error\": \"Name and type are required\"}";
        res.json(allocator, error_json);
        return;
    }

    // Update datasource
    auth.database.updateDatasource(db, datasource_id, data.name, data.type, data.config) catch |err| {
        std.log.err("Failed to update datasource: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to update datasource\"}";
        res.json(allocator, error_json);
        return;
    };

    const response_json = "{\"success\": true, \"message\": \"Datasource updated\"}";
    res.json(allocator, response_json);
}

fn deleteDatasourceHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, db) orelse return;

    // Parse project ID and datasource ID from path
    const path_parts = std.mem.splitScalar(u8, req.path, '/');
    var iter = path_parts;
    _ = iter.next(); // api
    _ = iter.next(); // projects
    const project_id = iter.next() orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid project ID\"}";
        res.json(allocator, error_json);
        return;
    };
    _ = iter.next(); // datasources
    const datasource_id = iter.next() orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid datasource ID\"}";
        res.json(allocator, error_json);
        return;
    };

    // Check project ownership
    const project = auth.database.getProjectById(allocator, db, project_id) catch {
        res.status_code = 404;
        const error_json = "{\"error\": \"Project not found\"}";
        res.json(allocator, error_json);
        return;
    };
    defer {
        allocator.free(project.id);
        allocator.free(project.user_id);
        allocator.free(project.name);
        allocator.free(project.description);
        allocator.free(project.created_at);
    }

    if (!std.mem.eql(u8, project.user_id, user.id)) {
        res.status_code = 403;
        const error_json = "{\"error\": \"Access denied\"}";
        res.json(allocator, error_json);
        return;
    }

    // Delete datasource
    auth.database.deleteDatasource(db, datasource_id) catch |err| {
        std.log.err("Failed to delete datasource: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to delete datasource\"}";
        res.json(allocator, error_json);
        return;
    };

    const response_json = "{\"success\": true, \"message\": \"Datasource deleted\"}";
    res.json(allocator, response_json);
}

fn requireRootUser(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) ?auth.User {
    const user = getUserFromSession(req, res, allocator, db) orelse return null;

    // Check if user is root
    if (!std.mem.eql(u8, user.username, "root")) {
        res.status_code = 403;
        const error_json = "{\"error\": \"Access denied. Root user required.\"}";
        res.json(allocator, error_json);
        return null;
    }

    return user;
}

fn getUserFromSession(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) ?auth.User {
    // Get session token from cookie
    const cookie_header = req.headers.get("Cookie") orelse req.headers.get("cookie");
    const cookie_header_unwrapped = cookie_header orelse {
        res.status_code = 401;
        const error_json = "{\"error\": \"No session found\"}";
        res.json(allocator, error_json);
        return null;
    };

    var token: ?[]const u8 = null;
    var cookie_iter = std.mem.tokenizeScalar(u8, cookie_header_unwrapped, ';');
    while (cookie_iter.next()) |cookie| {
        const trimmed = std.mem.trim(u8, cookie, " ");
        if (std.mem.startsWith(u8, trimmed, "session_token=")) {
            token = trimmed["session_token=".len..];
            break;
        }
    }

    const session_token = token orelse {
        res.status_code = 401;
        const error_json = "{\"error\": \"No session token found\"}";
        res.json(allocator, error_json);
        return null;
    };

    // Validate session and get user
    const user = auth.validateSession(allocator, db, session_token) catch |err| switch (err) {
        error.InvalidToken => {
            res.status_code = 401;
            const error_json = "{\"error\": \"Invalid session\"}";
            res.json(allocator, error_json);
            return null;
        },
        else => {
            res.status_code = 500;
            const error_json = "{\"error\": \"Failed to validate session\"}";
            res.json(allocator, error_json);
            return null;
        },
    };

    return user;
}

fn profileHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get session token from cookie - optimized parsing
    const session_token = extractSessionToken(req.headers, allocator) catch |err| switch (err) {
        error.NoCookie => {
            res.status_code = 401;
            const error_json = "{\"error\": \"No session found\"}";
            res.json(allocator, error_json);
            return;
        },
        error.NoToken => {
            res.status_code = 401;
            const error_json = "{\"error\": \"No session token found\"}";
            res.json(allocator, error_json);
            return;
        },
        error.OutOfMemory => {
            res.status_code = 500;
            const error_json = "{\"error\": \"Memory allocation failed\"}";
            res.json(allocator, error_json);
            return;
        },
    };

    // Validate session and get user
    const user = auth.validateSession(allocator, db, session_token) catch |err| {
        std.log.err("Session validation failed with error: {}", .{err});
        switch (err) {
            error.InvalidToken => {
                res.status_code = 401;
                const error_json = "{\"error\": \"Invalid session\"}";
                res.json(allocator, error_json);
                return;
            },
            else => {
                res.status_code = 500;
                const error_json = "{\"error\": \"Failed to validate session\"}";
                res.json(allocator, error_json);
                return;
            },
        }
    };
    defer {
        allocator.free(user.id);
        allocator.free(user.username);
        allocator.free(user.password_hash);
        allocator.free(user.created_at);
    }

    // Return user profile - pre-allocate response buffer
    var response_buffer = std.ArrayList(u8).initCapacity(allocator, 200) catch unreachable;
    defer response_buffer.deinit(allocator);

    response_buffer.writer(allocator).print(
        \\{{"success": true, "user": {{ "id": "{s}", "username": "{s}", "created_at": "{s}" }}}}
    , .{ user.id, user.username, user.created_at }) catch {
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create response\"}";
        res.json(allocator, error_json);
        return;
    };

    res.json(allocator, response_buffer.items);
}

// Optimized session token extraction
fn extractSessionToken(headers: std.StringHashMap([]const u8), allocator: std.mem.Allocator) ![]const u8 {
    const cookie_header = headers.get("Cookie") orelse headers.get("cookie") orelse return error.NoCookie;

    // Fast path: check for session_token at start
    if (std.mem.startsWith(u8, cookie_header, "session_token=")) {
        const end = std.mem.indexOfScalar(u8, cookie_header, ';') orelse cookie_header.len;
        return allocator.dupe(u8, cookie_header["session_token=".len..end]);
    }

    // Slow path: parse all cookies
    var cookie_iter = std.mem.tokenizeScalar(u8, cookie_header, ';');
    while (cookie_iter.next()) |cookie| {
        const trimmed = std.mem.trim(u8, cookie, " ");
        if (std.mem.startsWith(u8, trimmed, "session_token=")) {
            return allocator.dupe(u8, trimmed["session_token=".len..]);
        }
    }

    return error.NoToken;
}

const WebSocketFrame = struct {
    fin: bool,
    opcode: u8,
    mask: bool,
    payload_len: u64,
    mask_key: ?[4]u8,
    payload: []const u8,
};

const WebSocketConnection = struct {
    stream: net.Stream,
    allocator: std.mem.Allocator,

    fn readFrame(self: *WebSocketConnection, buffer: []u8) !WebSocketFrame {
        const header_len = try self.stream.read(buffer[0..2]);
        if (header_len < 2) return error.ConnectionClosed;

        const first_byte = buffer[0];
        const second_byte = buffer[1];

        const fin = (first_byte & 0x80) != 0;
        const opcode = first_byte & 0x0F;
        const mask = (second_byte & 0x80) != 0;
        var payload_len: u64 = second_byte & 0x7F;

        var offset: usize = 2;

        if (payload_len == 126) {
            const len_bytes = try self.stream.read(buffer[offset .. offset + 2]);
            if (len_bytes < 2) return error.InvalidFrame;
            payload_len = std.mem.readInt(u16, buffer[offset .. offset + 2][0..2], .big);
            offset += 2;
        } else if (payload_len == 127) {
            const len_bytes = try self.stream.read(buffer[offset .. offset + 8]);
            if (len_bytes < 8) return error.InvalidFrame;
            payload_len = std.mem.readInt(u64, buffer[offset .. offset + 8][0..8], .big);
            offset += 8;
        }

        var mask_key: ?[4]u8 = null;
        if (mask) {
            const mask_bytes = try self.stream.read(buffer[offset .. offset + 4]);
            if (mask_bytes < 4) return error.InvalidFrame;
            mask_key = buffer[offset .. offset + 4][0..4].*;
            offset += 4;
        }

        if (payload_len > buffer.len - offset) return error.PayloadTooLarge;
        const payload_bytes = try self.stream.read(buffer[offset .. offset + @as(usize, @intCast(payload_len))]);
        if (payload_bytes < @as(usize, @intCast(payload_len))) return error.InvalidFrame;

        var payload = buffer[offset .. offset + @as(usize, @intCast(payload_len))];
        if (mask) {
            for (payload, 0..) |byte, i| {
                payload[i] = byte ^ mask_key.?[i % 4];
            }
        }

        return WebSocketFrame{
            .fin = fin,
            .opcode = opcode,
            .mask = mask,
            .payload_len = payload_len,
            .mask_key = mask_key,
            .payload = payload,
        };
    }

    fn writeFrame(self: *WebSocketConnection, opcode: u8, payload: []const u8) !void {
        var header: [10]u8 = undefined;
        var header_len: usize = 2;

        header[0] = 0x80 | opcode;

        if (payload.len < 126) {
            header[1] = @intCast(payload.len);
        } else if (payload.len < 65536) {
            header[1] = 126;
            std.mem.writeInt(u16, header[2..4], @intCast(payload.len), .big);
            header_len = 4;
        } else {
            header[1] = 127;
            std.mem.writeInt(u64, header[2..10], payload.len, .big);
            header_len = 10;
        }

        _ = try self.stream.writeAll(header[0..header_len]);
        if (payload.len > 0) {
            _ = try self.stream.writeAll(payload);
        }
    }

    fn sendText(self: *WebSocketConnection, text: []const u8) !void {
        try self.writeFrame(0x1, text);
    }

    fn close(self: *WebSocketConnection) !void {
        try self.writeFrame(0x8, &[_]u8{ 0x03, 0xE8 });
        self.stream.close();
    }
};

fn handleConnection(stream: net.Stream, allocator: std.mem.Allocator, db: *zlay_db.Database, router: *Router, production: bool) void {
    defer stream.close();

    var buffer: [4096]u8 = undefined;
    const bytes_read = stream.read(&buffer) catch return;

    if (bytes_read == 0) return;

    const request_data = buffer[0..bytes_read];
    var request = parseRequest(request_data, allocator) catch return;
    defer request.headers.deinit();

    if (isWebSocketUpgrade(&request)) {
        const websocket_key = request.headers.get("Sec-WebSocket-Key") orelse return;

        const accept_key = generateWebSocketAcceptKey(websocket_key, allocator) catch return;
        defer allocator.free(accept_key);

        var response = Response.init(allocator);
        defer response.headers.deinit();
        response.status_code = 101;
        response.headers.put("Upgrade", "websocket") catch {};
        response.headers.put("Connection", "Upgrade") catch {};
        response.headers.put("Sec-WebSocket-Accept", accept_key) catch {};

        const response_str = response.toString(allocator) catch return;
        defer allocator.free(response_str);

        _ = stream.writeAll(response_str) catch return;

        var ws_connection = WebSocketConnection{
            .stream = stream,
            .allocator = allocator,
        };

        if (!router.handleWebSocket(request.path, &ws_connection, allocator)) {
            ws_connection.close() catch {};
        }
    } else {
        var response = Response.init(allocator);
        defer response.headers.deinit();

        const handle_result = router.handle(request.method, request.path, &request, &response, allocator, db);
        if (!handle_result) {
            if (production) {
                // In production, try to serve static files or fallback to SPA
                if (std.mem.startsWith(u8, request.path, "/assets/") or
                    std.mem.eql(u8, request.path, "/favicon.ico"))
                {
                    const file_path = request.path[1..]; // Remove leading /
                    serveStaticFile(&request, &response, allocator, file_path);
                } else {
                    // Fallback to SPA for any other route
                    spaHandler(&request, &response, allocator);
                }
            } else {
                response.status_code = 404;
                const not_found = "{\"error\": \"Not Found\"}";
                response.json(allocator, not_found);
            }
        }

        const response_str = response.toString(allocator) catch return;
        defer allocator.free(response_str);

        _ = stream.writeAll(response_str) catch return;

        allocator.free(request.method);
        allocator.free(request.path);
        allocator.free(request.body);
        if (response.body.len > 0) {
            allocator.free(response.body);
        }
    }
}

fn generateSlug(allocator: std.mem.Allocator, name: []const u8) ![]const u8 {
    var slug = try std.ArrayList(u8).initCapacity(allocator, name.len);
    defer slug.deinit(allocator);

    for (name) |char| {
        if (std.ascii.isAlphanumeric(char)) {
            try slug.append(allocator, std.ascii.toLower(char));
        } else if (char == ' ' or char == '-' or char == '_') {
            try slug.append(allocator, '-');
        }
        // Skip other characters
    }

    // Remove consecutive hyphens and trim leading/trailing
    var cleaned = try std.ArrayList(u8).initCapacity(allocator, slug.items.len);
    defer cleaned.deinit(allocator);

    var prev_hyphen = false;
    for (slug.items) |char| {
        if (char == '-') {
            if (!prev_hyphen) {
                try cleaned.append(allocator, char);
                prev_hyphen = true;
            }
        } else {
            try cleaned.append(allocator, char);
            prev_hyphen = false;
        }
    }

    // Trim trailing hyphens
    while (cleaned.items.len > 0 and cleaned.items[cleaned.items.len - 1] == '-') {
        _ = cleaned.pop();
    }

    // If empty, use default
    if (cleaned.items.len == 0) {
        return allocator.dupe(u8, "client");
    }

    return cleaned.toOwnedSlice(allocator);
}

pub fn main() !void {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const base_allocator = gpa.allocator();

    // Use thread-safe allocator for concurrent requests
    var thread_safe = std.heap.ThreadSafeAllocator{ .child_allocator = base_allocator };
    const allocator = thread_safe.allocator();

    std.debug.print("Starting server...\n", .{});

    // Initialize rate limiter
    RateLimiter.init();
    defer RateLimiter.deinit();

    // Load environment variables
    var env_map = try loadEnv(base_allocator);
    defer {
        var it = env_map.iterator();
        while (it.next()) |entry| {
            base_allocator.free(entry.key_ptr.*);
            base_allocator.free(entry.value_ptr.*);
        }
        env_map.deinit();
    }

    // TODO: Set environment variables from .env file
    // For now, run with: DATABASE_URL=test.db ./zig-out/bin/zlay-backend

    // Configure database using zlay-db
    var config = try db_config.DatabaseConfig.fromEnvironment(base_allocator);
    defer config.deinit();

    const zlay_config = config.toZlayDbConfig();
    var db = try zlay_db.Database.connect(zlay_config);
    defer db.close();

    // Log database connection details (without password)
    const db_type_str = switch (config.database_type) {
        .postgresql => "postgresql",
        .mysql => "mysql",
        .sqlite => "sqlite",
        else => "unknown",
    };
    if (config.database_type == .sqlite) {
        std.log.info("Connected to {s} database: {s}", .{ db_type_str, config.file_path orelse "unknown" });
    } else {
        std.log.info("Connected to {s} database: {s}:{d}/{s}", .{ db_type_str, config.host orelse "localhost", config.port orelse 5432, config.database orelse "unknown" });
    }

    std.log.info("Connected to database using zlay-db!", .{});

    // Check if we're in production mode
    const is_prod_result = blk: {
        const result = std.process.getEnvVarOwned(base_allocator, "NODE_ENV") catch |err| {
            if (err == error.EnvironmentVariableNotFound) {
                break :blk try base_allocator.dupe(u8, "");
            }
            return err;
        };
        break :blk result;
    };
    defer base_allocator.free(is_prod_result);
    const production = std.mem.eql(u8, is_prod_result, "production");

    const port: u16 = if (production) 3000 else 8080;

    var router = Router.init(base_allocator);

    // Add authentication routes (available in both dev and prod)
    router.getSimple(base_allocator, "/api/health", healthHandler);
    router.options(base_allocator, "/api/auth/register", corsHandler);
    router.post(base_allocator, "/api/auth/register", registerHandler);
    router.options(base_allocator, "/api/auth/login", corsHandler);
    router.post(base_allocator, "/api/auth/login", loginHandler);
    router.options(base_allocator, "/api/auth/logout", corsHandler);
    router.post(base_allocator, "/api/auth/logout", logoutHandler);
    router.options(base_allocator, "/api/auth/profile", corsHandler);
    router.get(base_allocator, "/api/auth/profile", profileHandler);

    // Add project routes
    router.options(base_allocator, "/api/projects", corsHandler);
    router.get(base_allocator, "/api/projects", getProjectsHandler);
    router.post(base_allocator, "/api/projects", createProjectHandler);
    router.options(base_allocator, "/api/projects/*", corsHandler);
    router.get(base_allocator, "/api/projects/*", getProjectHandler);
    router.put(base_allocator, "/api/projects/*", updateProjectHandler);
    router.delete(base_allocator, "/api/projects/*", deleteProjectHandler);

    // Add datasource routes
    router.options(base_allocator, "/api/projects/*/datasources", corsHandler);
    router.get(base_allocator, "/api/projects/*/datasources", getDatasourcesHandler);
    router.post(base_allocator, "/api/projects/*/datasources", createDatasourceHandler);
    router.options(base_allocator, "/api/projects/*/datasources/*", corsHandler);
    router.get(base_allocator, "/api/projects/*/datasources/*", getDatasourceHandler);
    router.put(base_allocator, "/api/projects/*/datasources/*", updateDatasourceHandler);
    router.delete(base_allocator, "/api/projects/*/datasources/*", deleteDatasourceHandler);

    // Add admin routes (root user only)
    router.options(base_allocator, "/api/admin/clients", corsHandler);
    router.get(base_allocator, "/api/admin/clients", getClientsHandler);
    router.post(base_allocator, "/api/admin/clients", createClientHandler);
    // Register routes in order of specificity (more specific first)
    router.options(base_allocator, "/api/admin/clients/*/domains/*", corsHandler);
    router.delete(base_allocator, "/api/admin/clients/*/domains/*", removeDomainHandler);
    router.put(base_allocator, "/api/admin/clients/*/domains/*", updateDomainHandler);
    router.options(base_allocator, "/api/admin/clients/*/domains", corsHandler);
    router.get(base_allocator, "/api/admin/clients/*/domains", getClientDomainsHandler);
    router.post(base_allocator, "/api/admin/clients/*/domains", addDomainHandler);
    router.options(base_allocator, "/api/admin/clients/*", corsHandler);
    router.put(base_allocator, "/api/admin/clients/*", updateClientHandler);
    router.delete(base_allocator, "/api/admin/clients/*", deleteClientHandler);

    if (production) {
        // In production, serve the SPA
        router.getSimple(base_allocator, "/", spaHandler);
        router.getSimple(base_allocator, "/api", apiHandler);
        router.websocket(base_allocator, "/ws", websocketHandler);
    } else {
        // In development, use the original handlers
        router.getSimple(base_allocator, "/", homeHandler);
        router.getSimple(base_allocator, "/api", apiHandler);
        router.websocket(base_allocator, "/ws", websocketHandler);
    }

    const address = try net.Address.parseIp("127.0.0.1", port);
    var listener = try address.listen(.{ .reuse_address = true });
    defer listener.deinit();

    std.debug.print("Server listening on http://127.0.0.1:{d}\n", .{port});
    std.debug.print("WebSocket endpoint: ws://127.0.0.1:{d}/ws\n", .{port});
    if (production) {
        std.debug.print("Production mode: Serving SPA from ../frontend/dist\n", .{});
    }

    while (true) {
        const connection = try listener.accept();

        // Handle each connection in a separate thread for concurrency
        const thread = try std.Thread.spawn(.{}, handleConnection, .{ connection.stream, allocator, &db, &router, production });
        thread.detach();
    }
}

fn loadEnv(allocator: std.mem.Allocator) !std.StringHashMap([]const u8) {
    var env_map = std.StringHashMap([]const u8).init(allocator);

    // Try to read .env file
    if (std.fs.cwd().openFile(".env", .{})) |file| {
        defer file.close();
        const contents = try file.readToEndAlloc(allocator, 1024 * 1024);
        defer allocator.free(contents);

        var lines = std.mem.tokenizeScalar(u8, contents, '\n');
        while (lines.next()) |line| {
            if (line.len == 0 or line[0] == '#') continue;

            if (std.mem.indexOfScalar(u8, line, '=')) |eq_pos| {
                const key = line[0..eq_pos];
                const value = line[eq_pos + 1 ..];

                const trimmed_key = std.mem.trim(u8, key, " \t\r");
                const trimmed_value = std.mem.trim(u8, value, " \t\r");

                const key_copy = try allocator.dupe(u8, trimmed_key);
                const value_copy = try allocator.dupe(u8, trimmed_value);
                try env_map.put(key_copy, value_copy);
            }
        }
    } else |err| switch (err) {
        error.FileNotFound => {
            std.log.warn("No .env file found, using system environment", .{});
        },
        else => return err,
    }

    return env_map;
}

// Admin handlers for root user
fn getClientsHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Require root user
    const user = requireRootUser(req, res, allocator, db) orelse return;
    defer {
        allocator.free(user.id);
        allocator.free(user.username);
        allocator.free(user.password_hash);
        allocator.free(user.created_at);
    }

    // Get all clients
    const clients = auth.database.getAllClients(allocator, db) catch |err| {
        std.log.err("Failed to get clients: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to get clients\"}";
        res.json(allocator, error_json);
        return;
    };
    defer {
        for (clients) |client| {
            allocator.free(client.id);
            allocator.free(client.name);
            allocator.free(client.slug);
            if (client.ai_api_key) |key| allocator.free(key);
            if (client.ai_api_url) |url| allocator.free(url);
            if (client.ai_api_model) |model| allocator.free(model);
            allocator.free(client.created_at);
        }
        allocator.free(clients);
    }

    // Build JSON response
    var json = std.ArrayList(u8).initCapacity(allocator, 2048) catch unreachable;
    defer json.deinit(allocator);
    json.writer(allocator).writeAll("[") catch {};

    for (clients, 0..) |client, i| {
        if (i > 0) json.writer(allocator).writeAll(",") catch {};
        json.writer(allocator).print(
            \\{{"id":"{s}","name":"{s}","slug":"{s}","ai_api_key":{s},"ai_api_url":{s},"ai_api_model":{s},"is_active":{},"created_at":"{s}"}}
        , .{
            client.id,
            client.name,
            client.slug,
            if (client.ai_api_key) |key| std.fmt.allocPrint(allocator, "\"{s}\"", .{key}) catch "\"\"" else "null",
            if (client.ai_api_url) |url| std.fmt.allocPrint(allocator, "\"{s}\"", .{url}) catch "\"\"" else "null",
            if (client.ai_api_model) |model| std.fmt.allocPrint(allocator, "\"{s}\"", .{model}) catch "\"\"" else "null",
            client.is_active,
            client.created_at,
        }) catch {};
    }
    json.writer(allocator).writeAll("]") catch {};

    res.json(allocator, json.items);
}

fn createClientHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    std.log.info("Create client request received", .{});
    // Add CORS headers
    setCorsHeaders(res, req);

    // Require root user
    const user = requireRootUser(req, res, allocator, db) orelse return;
    defer {
        allocator.free(user.id);
        allocator.free(user.username);
        allocator.free(user.password_hash);
        allocator.free(user.created_at);
    }

    // Parse JSON body
    std.log.info("Request body: '{s}'", .{req.body});
    const json_str_trimmed = std.mem.trim(u8, req.body, " \t\r\n");
    std.log.info("Trimmed body: '{s}'", .{json_str_trimmed});
    var parsed = std.json.parseFromSlice(struct {
        name: []const u8,
        ai_api_key: ?[]const u8,
        ai_api_url: ?[]const u8,
        ai_api_model: ?[]const u8,
    }, allocator, json_str_trimmed, .{}) catch {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid JSON format\"}";
        res.json(allocator, error_json);
        return;
    };
    defer parsed.deinit();

    const data = parsed.value;

    // Validate required fields
    if (data.name.len == 0) {
        res.status_code = 400;
        const error_json = "{\"error\": \"Name is required\"}";
        res.json(allocator, error_json);
        return;
    }

    // Generate slug from name
    const slug = generateSlug(allocator, data.name) catch {
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to generate slug\"}";
        res.json(allocator, error_json);
        return;
    };
    defer allocator.free(slug);

    // Create client
    const client_id = auth.database.createClient(allocator, db, data.name, slug, data.ai_api_key, data.ai_api_url, data.ai_api_model) catch |err| {
        std.log.err("Failed to create client: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create client\"}";
        res.json(allocator, error_json);
        return;
    };
    defer allocator.free(client_id);

    // Return success response
    const response_json = std.fmt.allocPrint(allocator,
        \\{{"success": true, "message": "Client created successfully", "client_id": "{s}"}}
    , .{client_id}) catch {
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create response\"}";
        res.json(allocator, error_json);
        return;
    };
    defer allocator.free(response_json);

    res.status_code = 201;
    res.json(allocator, response_json);
}

fn updateClientHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Require root user
    const user = requireRootUser(req, res, allocator, db) orelse return;
    defer {
        allocator.free(user.id);
        allocator.free(user.username);
        allocator.free(user.password_hash);
        allocator.free(user.created_at);
    }

    // Parse client ID from path
    const path_parts = std.mem.splitScalar(u8, req.path, '/');
    var iter = path_parts;
    _ = iter.next(); // skip empty
    _ = iter.next(); // api
    _ = iter.next(); // admin
    _ = iter.next(); // clients
    const client_id = iter.next() orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid client ID\"}";
        res.json(allocator, error_json);
        return;
    };

    // Parse JSON body
    const json_str_trimmed = std.mem.trim(u8, req.body, " \t\r\n");
    std.log.info("Client update body: '{s}' len: {}", .{ json_str_trimmed, json_str_trimmed.len });
    var parsed = std.json.parseFromSlice(std.json.Value, allocator, json_str_trimmed, .{}) catch |err| {
        std.log.info("Parse error: {}", .{err});
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid JSON format\"}";
        res.json(allocator, error_json);
        return;
    };
    defer parsed.deinit();

    const data = parsed.value;

    // Fetch current client
    const current_client = auth.database.getClientById(allocator, db, client_id) catch |err| {
        std.log.err("Failed to get current client: {}", .{err});
        res.status_code = 404;
        const error_json = "{\"error\": \"Client not found\"}";
        res.json(allocator, error_json);
        return;
    };
    defer {
        allocator.free(current_client.id);
        allocator.free(current_client.name);
        allocator.free(current_client.slug);
        if (current_client.ai_api_key) |key| allocator.free(key);
        if (current_client.ai_api_url) |url| allocator.free(url);
        if (current_client.ai_api_model) |model| allocator.free(model);
        allocator.free(current_client.created_at);
    }

    // Get fields, default to current values if not provided
    const name = if (data.object.get("name")) |v| if (v == .null) current_client.name else v.string else current_client.name;
    const slug = if (data.object.get("slug")) |v| if (v == .null) current_client.slug else v.string else current_client.slug;
    const ai_api_key = if (data.object.get("ai_api_key")) |v| if (v == .null) null else if (v == .string) allocator.dupe(u8, v.string) catch null else current_client.ai_api_key else current_client.ai_api_key;
    const ai_api_url = if (data.object.get("ai_api_url")) |v| if (v == .null) null else if (v == .string) allocator.dupe(u8, v.string) catch null else current_client.ai_api_url else current_client.ai_api_url;
    const ai_api_model = if (data.object.get("ai_api_model")) |v| if (v == .null) null else if (v == .string) allocator.dupe(u8, v.string) catch null else current_client.ai_api_model else current_client.ai_api_model;
    const is_active = if (data.object.get("is_active")) |v| v.bool else current_client.is_active;

    // Validate
    if (name.len == 0 or slug.len == 0) {
        res.status_code = 400;
        const error_json = "{\"error\": \"Name and slug cannot be empty\"}";
        res.json(allocator, error_json);
        return;
    }

    // Update client
    auth.database.updateClient(db, client_id, name, slug, ai_api_key, ai_api_url, ai_api_model, is_active) catch |err| {
        std.log.err("Failed to update client: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to update client\"}";
        res.json(allocator, error_json);
        return;
    };

    const response_json = "{\"success\": true, \"message\": \"Client updated\"}";
    res.json(allocator, response_json);
}

fn deleteClientHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Require root user
    const user = requireRootUser(req, res, allocator, db) orelse return;
    defer {
        allocator.free(user.id);
        allocator.free(user.username);
        allocator.free(user.password_hash);
        allocator.free(user.created_at);
    }

    // Parse client ID from path
    const path_parts = std.mem.splitScalar(u8, req.path, '/');
    var iter = path_parts;
    _ = iter.next(); // skip empty
    _ = iter.next(); // api
    _ = iter.next(); // admin
    _ = iter.next(); // clients
    const client_id = iter.next() orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid client ID\"}";
        res.json(allocator, error_json);
        return;
    };

    // Check if client exists and is active
    const client = auth.database.getClientById(allocator, db, client_id) catch |err| {
        std.log.err("Failed to get client: {}", .{err});
        res.status_code = 404;
        const error_json = "{\"error\": \"Client not found\"}";
        res.json(allocator, error_json);
        return;
    };
    defer {
        allocator.free(client.id);
        allocator.free(client.name);
        allocator.free(client.slug);
        if (client.ai_api_key) |key| allocator.free(key);
        if (client.ai_api_url) |url| allocator.free(url);
        if (client.ai_api_model) |model| allocator.free(model);
        allocator.free(client.created_at);
    }

    // Delete client (hard delete)
    auth.database.deleteClient(db, client_id) catch |err| {
        std.log.err("Failed to delete client: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to delete client\"}";
        res.json(allocator, error_json);
        return;
    };

    const response_json = "{\"success\": true, \"message\": \"Client deleted\"}";
    res.json(allocator, response_json);
}

fn getClientDomainsHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Require root user
    const user = requireRootUser(req, res, allocator, db) orelse return;
    defer {
        allocator.free(user.id);
        allocator.free(user.username);
        allocator.free(user.password_hash);
        allocator.free(user.created_at);
    }

    // Parse client ID from path
    const path_parts = std.mem.splitScalar(u8, req.path, '/');
    var iter = path_parts;
    _ = iter.next(); // skip empty
    _ = iter.next(); // api
    _ = iter.next(); // admin
    _ = iter.next(); // clients
    const client_id = iter.next() orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid client ID\"}";
        res.json(allocator, error_json);
        return;
    };
    _ = iter.next(); // domains

    std.debug.print("DEBUG: client_id = {s}\n", .{client_id});

    // Get domains
    std.debug.print("DEBUG: calling getDomainsByClient with client_id = {s}\n", .{client_id});
    const domains = auth.database.getDomainsByClient(allocator, db, client_id) catch |err| {
        std.log.err("Failed to get domains: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to get domains\"}";
        res.json(allocator, error_json);
        return;
    };
    defer {
        for (domains) |domain| {
            allocator.free(domain.id);
            allocator.free(domain.client_id);
            allocator.free(domain.domain);
            allocator.free(domain.created_at);
        }
        allocator.free(domains);
    }

    // Build JSON response
    var json = std.ArrayList(u8).initCapacity(allocator, 1024) catch unreachable;
    defer json.deinit(allocator);
    json.writer(allocator).writeAll("[") catch {};

    for (domains, 0..) |domain, i| {
        if (i > 0) json.writer(allocator).writeAll(",") catch {};
        json.writer(allocator).print(
            \\{{"id":"{s}","client_id":"{s}","domain":"{s}","is_active":{},"created_at":"{s}"}}
        , .{ domain.id, domain.client_id, domain.domain, domain.is_active, domain.created_at }) catch {};
    }
    json.writer(allocator).writeAll("]") catch {};

    res.json(allocator, json.items);
}

fn addDomainHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Require root user
    const user = requireRootUser(req, res, allocator, db) orelse return;
    defer {
        allocator.free(user.id);
        allocator.free(user.username);
        allocator.free(user.password_hash);
        allocator.free(user.created_at);
    }

    // Parse client ID from path
    const path_parts = std.mem.splitScalar(u8, req.path, '/');
    var iter = path_parts;
    _ = iter.next(); // skip empty
    _ = iter.next(); // api
    _ = iter.next(); // admin
    _ = iter.next(); // clients
    const client_id = iter.next() orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid client ID\"}";
        res.json(allocator, error_json);
        return;
    };
    _ = iter.next(); // domains

    // Parse JSON body
    const json_str = req.body;
    var parsed = std.json.parseFromSlice(struct {
        domain: []const u8,
    }, allocator, json_str, .{}) catch {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid JSON format\"}";
        res.json(allocator, error_json);
        return;
    };
    defer parsed.deinit();

    const data = parsed.value;

    // Validate domain
    if (data.domain.len == 0) {
        res.status_code = 400;
        const error_json = "{\"error\": \"Domain is required\"}";
        res.json(allocator, error_json);
        return;
    }

    // Add domain
    const domain_id = auth.database.addDomainToClient(allocator, db, client_id, data.domain) catch |err| {
        std.log.err("Failed to add domain: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to add domain\"}";
        res.json(allocator, error_json);
        return;
    };
    defer allocator.free(domain_id);

    // Return success response
    const response_json = std.fmt.allocPrint(allocator,
        \\{{"success": true, "message": "Domain added successfully", "domain_id": "{s}"}}
    , .{domain_id}) catch {
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create response\"}";
        res.json(allocator, error_json);
        return;
    };
    defer allocator.free(response_json);

    res.status_code = 201;
    res.json(allocator, response_json);
}

fn removeDomainHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Require root user
    const user = requireRootUser(req, res, allocator, db) orelse return;
    defer {
        allocator.free(user.id);
        allocator.free(user.username);
        allocator.free(user.password_hash);
        allocator.free(user.created_at);
    }

    // Parse domain ID from path
    const path_parts = std.mem.splitScalar(u8, req.path, '/');
    var iter = path_parts;
    _ = iter.next(); // skip empty
    _ = iter.next(); // api
    _ = iter.next(); // admin
    _ = iter.next(); // clients
    _ = iter.next(); // client_id
    _ = iter.next(); // domains
    const domain_id = iter.next() orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid domain ID\"}";
        res.json(allocator, error_json);
        return;
    };

    // Remove domain
    auth.database.removeDomain(db, domain_id) catch |err| {
        std.log.err("Failed to remove domain: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to remove domain\"}";
        res.json(allocator, error_json);
        return;
    };

    const response_json = "{\"success\": true, \"message\": \"Domain removed\"}";
    res.json(allocator, response_json);
}

fn updateDomainHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, db: *zlay_db.Database) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Require root user
    const user = requireRootUser(req, res, allocator, db) orelse return;
    defer {
        allocator.free(user.id);
        allocator.free(user.username);
        allocator.free(user.password_hash);
        allocator.free(user.created_at);
    }

    // Parse domain ID from path
    const path_parts = std.mem.splitScalar(u8, req.path, '/');
    var iter = path_parts;
    _ = iter.next(); // skip empty
    _ = iter.next(); // api
    _ = iter.next(); // admin
    _ = iter.next(); // clients
    _ = iter.next(); // client_id
    _ = iter.next(); // domains
    const domain_id = iter.next() orelse {
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid domain ID\"}";
        res.json(allocator, error_json);
        return;
    };

    // Parse JSON body
    const json_str_trimmed = std.mem.trim(u8, req.body, " \t\r\n");
    std.log.info("Domain update body: '{s}' len: {}", .{ json_str_trimmed, json_str_trimmed.len });
    var parsed = std.json.parseFromSlice(std.json.Value, allocator, json_str_trimmed, .{}) catch |err| {
        std.log.info("Parse error: {}", .{err});
        res.status_code = 400;
        const error_json = "{\"error\": \"Invalid JSON format\"}";
        res.json(allocator, error_json);
        return;
    };
    defer parsed.deinit();

    const data = parsed.value;
    const is_active = data.object.get("is_active").?.bool;

    // Update domain
    auth.database.updateDomain(db, domain_id, is_active) catch |err| {
        std.log.err("Failed to update domain: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to update domain\"}";
        res.json(allocator, error_json);
        return;
    };

    const response_json = "{\"success\": true, \"message\": \"Domain updated\"}";
    res.json(allocator, response_json);
}

fn websocketHandler(ws: *WebSocketConnection, allocator: std.mem.Allocator) void {
    ws.sendText("Connected to WebSocket server!") catch |err| {
        print("Failed to send welcome message: {}\n", .{err});
        return;
    };

    var buffer: [4096]u8 = undefined;
    while (true) {
        const frame = ws.readFrame(&buffer) catch |err| {
            print("WebSocket error: {}\n", .{err});
            break;
        };

        switch (frame.opcode) {
            0x1 => { // Text frame
                const message = frame.payload;
                print("Received: {s}\n", .{message});

                var response = std.ArrayList(u8).initCapacity(allocator, 128) catch unreachable;
                defer response.deinit(allocator);
                response.writer(allocator).print("Echo: {s}", .{message}) catch {};

                ws.sendText(response.items) catch |err| {
                    print("Failed to send echo: {}\n", .{err});
                    break;
                };
            },
            0x8 => { // Close frame
                print("Client requested close\n", .{});
                break;
            },
            0x9 => { // Ping frame
                ws.writeFrame(0xA, frame.payload) catch |err| {
                    print("Failed to send pong: {}\n", .{err});
                    break;
                };
            },
            else => {
                print("Unknown opcode: {x}\n", .{frame.opcode});
            },
        }
    }

    ws.close() catch {};
}
