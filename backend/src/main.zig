const std = @import("std");
const net = std.net;
const print = std.debug.print;
const crypto = std.crypto;
const base64 = std.base64;
const pg = @import("pg");
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

fn setCorsHeaders(res: *Response, req: *Request) void {
    const origin = req.headers.get("Origin") orelse req.headers.get("origin") orelse "http://localhost:5173";
    res.headers.put("Access-Control-Allow-Origin", origin) catch {};
    res.headers.put("Access-Control-Allow-Credentials", "true") catch {};
    res.headers.put("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS") catch {};
    res.headers.put("Access-Control-Allow-Headers", "Content-Type, Authorization") catch {};
}

const Router = struct {
    const HandlerFn = *const fn (*Request, *Response, std.mem.Allocator, *pg.Pool) void;
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

    fn handle(self: *Router, method: []const u8, path: []const u8, req: *const Request, res: *Response, allocator: std.mem.Allocator, pool: *pg.Pool) bool {

        // Try exact match first for auth routes (with pool)
        var exact_route = std.ArrayList(u8).initCapacity(allocator, 64) catch unreachable;
        defer exact_route.deinit(allocator);
        exact_route.writer(allocator).print("{s} {s}", .{ method, path }) catch {};

        // Debug: Print the route we're looking for
        std.debug.print("Looking for route: {s}\n", .{exact_route.items});

        if (self.routes.get(exact_route.items)) |handler| {
            std.debug.print("Found handler for route: {s}\n", .{exact_route.items});

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

            handler(@constCast(req), res, allocator, pool);
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
            const route_path = entry.key_ptr.*;
            if (std.mem.indexOf(u8, route_path, "/*")) |wildcard_pos| {
                const route_prefix = route_path[0..wildcard_pos];
                const method_prefix = route_prefix[0..std.mem.indexOfScalar(u8, route_prefix, ' ').?];
                const path_prefix = route_prefix[std.mem.indexOfScalar(u8, route_prefix, ' ').? + 1 ..];

                if (std.mem.eql(u8, method, method_prefix) and
                    std.mem.startsWith(u8, path, path_prefix))
                {
                    const handler = entry.value_ptr.*;
                    handler(@constCast(req), res, allocator, pool);
                    return true;
                }
            }
        }

        return false;
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
fn getClientId(req: *Request, allocator: std.mem.Allocator, pool: *pg.Pool) ?[]const u8 {
    // Try to get client_id from X-Client-ID header first
    if (req.headers.get("X-Client-ID")) |client_id_header| {
        return allocator.dupe(u8, client_id_header) catch null;
    }

    // Try to get client from subdomain (e.g., client.example.com)
    if (req.headers.get("Host")) |host| {
        if (std.mem.indexOf(u8, host, ".")) |dot_pos| {
            const subdomain = host[0..dot_pos];
            if (subdomain.len > 0) {
                // Look up client by slug/name
                const client_result = pool.query(
                    \\SELECT id::text FROM clients WHERE slug = $1 AND is_active = true LIMIT 1
                , .{subdomain}) catch return null;
                defer client_result.deinit();

                if (client_result.next() catch null) |row| {
                    return allocator.dupe(u8, row.get([]const u8, 0)) catch null;
                }
            }
        }
    }

    // Default to first active client for development
    std.log.info("Looking for default active client", .{});
    const default_client_result = pool.query(
        \\SELECT id::text FROM clients WHERE is_active = true ORDER BY created_at ASC LIMIT 1
    , .{}) catch |err| {
        std.log.err("Failed to query default client: {}", .{err});
        return null;
    };
    defer default_client_result.deinit();

    if (default_client_result.next() catch null) |row| {
        const client_id = allocator.dupe(u8, row.get([]const u8, 0)) catch null;
        std.log.info("Found default client: {s}", .{client_id orelse "null"});
        return client_id;
    }

    std.log.err("No active client found", .{});
    return null;
}

fn loginHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, pool: *pg.Pool) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get client ID
    const client_id = getClientId(req, allocator, pool) orelse {
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

    const user_data = parsed.value;

    // Validate input
    if (user_data.username.len < 3 or user_data.password.len < 8) {
        res.status_code = 400;
        const error_json = "{\"error\": \"Username must be at least 3 characters and password must be at least 8 characters\"}";
        res.json(allocator, error_json);
        return;
    }

    // Login user
    const login_req = auth.LoginRequest{
        .client_id = client_id,
        .username = user_data.username,
        .password = user_data.password,
    };

    const result = auth.loginUser(allocator, pool, login_req) catch |err| {
        std.log.err("Login failed with error: {}", .{err});
        switch (err) {
            error.UserNotFound => {
                std.log.info("Login failed: User '{s}' not found for client '{s}'", .{ user_data.username, client_id });
                res.status_code = 401;
                const error_json = "{\"error\": \"Invalid username or password\"}";
                res.json(allocator, error_json);
                return;
            },
            error.InvalidPassword => {
                std.log.info("Login failed: Invalid password for user '{s}'", .{user_data.username});
                res.status_code = 401;
                const error_json = "{\"error\": \"Invalid username or password\"}";
                res.json(allocator, error_json);
                return;
            },
            error.DatabaseError => {
                std.log.err("Login failed: Database error for user '{s}'", .{user_data.username});
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

    // Set session cookie
    const cookie_value = std.fmt.allocPrint(allocator, "session_token={s}; Domain=localhost; Path=/; Max-Age=86400", .{result.token}) catch {
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

fn registerHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, pool: *pg.Pool) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get client ID
    const client_id = getClientId(req, allocator, pool) orelse {
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

    const user_data = parsed.value;

    // Validate input
    if (user_data.username.len < 3 or user_data.password.len < 8) {
        res.status_code = 400;
        const error_json = "{\"error\": \"Username must be at least 3 characters and password must be at least 8 characters\"}";
        res.json(allocator, error_json);
        return;
    }

    // Register user
    const register_req = auth.RegisterRequest{
        .client_id = client_id,
        .username = user_data.username,
        .password = user_data.password,
    };

    const user = auth.registerUser(allocator, pool, register_req) catch |err| {
        std.log.err("Registration failed with error: {}", .{err});
        switch (err) {
            error.UserAlreadyExists => {
                std.log.info("Registration failed: User '{s}' already exists for client '{s}'", .{ user_data.username, client_id });
                res.status_code = 409;
                const error_json = "{\"error\": \"Username already exists\"}";
                res.json(allocator, error_json);
                return;
            },
            error.DatabaseError => {
                std.log.err("Registration failed: Database error for user '{s}'", .{user_data.username});
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
        .username = user_data.username,
        .password = user_data.password,
    };

    const result = auth.loginUser(allocator, pool, login_req) catch |err| {
        std.log.err("Auto-login after registration failed with error: {}", .{err});
        switch (err) {
            error.UserNotFound, error.InvalidPassword => {
                std.log.err("Auto-login failed: Unexpected error for newly created user '{s}'", .{user_data.username});
                res.status_code = 500;
                const error_json = "{\"error\": \"Registration successful but login failed\"}";
                res.json(allocator, error_json);
                return;
            },
            error.DatabaseError => {
                std.log.err("Auto-login failed: Database error for user '{s}'", .{user_data.username});
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

    // Set session cookie
    const cookie_value = std.fmt.allocPrint(allocator, "session_token={s}; Domain=localhost; Path=/; Max-Age=86400", .{result.token}) catch {
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create cookie\"}";
        res.json(allocator, error_json);
        return;
    };
    std.log.info("Setting cookie with token: {s}", .{result.token});
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

fn logoutHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, pool: *pg.Pool) void {
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
        auth.logoutUser(allocator, pool, session_token) catch {};
    }

    // Clear session cookie
    res.headers.put("Set-Cookie", "session_token=; HttpOnly; SameSite=Strict; Path=/; Max-Age=0") catch {};

    const success_json = "{\"success\": true, \"message\": \"Logout successful\"}";
    res.json(allocator, success_json);
}

fn corsHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, pool: *pg.Pool) void {
    _ = pool;
    // Add CORS headers for preflight requests
    setCorsHeaders(res, req);
    res.status_code = 200;
    res.json(allocator, "{}");
}

fn getProjectsHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, pool: *pg.Pool) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, pool) orelse return;

    // Get projects
    const projects = auth.database.getProjectsByUser(allocator, pool, user.id) catch |err| {
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

fn createProjectHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, pool: *pg.Pool) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, pool) orelse return;

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
    const project_id = auth.database.createProject(allocator, pool, user.id, data.name, data.description) catch |err| {
        std.log.err("Failed to create project: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create project\"}";
        res.json(allocator, error_json);
        return;
    };
    defer allocator.free(project_id);

    const response_json = std.fmt.allocPrint(allocator,
        \\{{"success": true, "message": "Project created", "project_id": "{s}"}}
    , .{project_id}) catch {
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create response\"}";
        res.json(allocator, error_json);
        return;
    };

    res.status_code = 201;
    res.json(allocator, response_json);
}

fn getProjectHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, pool: *pg.Pool) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, pool) orelse return;

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
    const project = auth.database.getProjectById(allocator, pool, project_id) catch |err| {
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

    const response_json = std.fmt.allocPrint(allocator,
        \\{{"id":"{s}","user_id":"{s}","name":"{s}","description":"{s}","is_active":{},"created_at":"{s}"}}
    , .{ project.id, project.user_id, project.name, project.description, project.is_active, project.created_at }) catch {
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create response\"}";
        res.json(allocator, error_json);
        return;
    };

    res.json(allocator, response_json);
}

fn updateProjectHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, pool: *pg.Pool) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, pool) orelse return;

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
    const project = auth.database.getProjectById(allocator, pool, project_id) catch {
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
    auth.database.updateProject(pool, project_id, data.name, data.description) catch |err| {
        std.log.err("Failed to update project: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to update project\"}";
        res.json(allocator, error_json);
        return;
    };

    const response_json = "{\"success\": true, \"message\": \"Project updated\"}";
    res.json(allocator, response_json);
}

fn deleteProjectHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, pool: *pg.Pool) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, pool) orelse return;

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
    const project = auth.database.getProjectById(allocator, pool, project_id) catch {
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
    auth.database.deleteProject(pool, project_id) catch |err| {
        std.log.err("Failed to delete project: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to delete project\"}";
        res.json(allocator, error_json);
        return;
    };

    const response_json = "{\"success\": true, \"message\": \"Project deleted\"}";
    res.json(allocator, response_json);
}

fn getDatasourcesHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, pool: *pg.Pool) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, pool) orelse return;

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
    const project = auth.database.getProjectById(allocator, pool, project_id) catch {
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
    const datasources = auth.database.getDatasourcesByProject(allocator, pool, project_id) catch |err| {
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

fn createDatasourceHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, pool: *pg.Pool) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, pool) orelse return;

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
    const project = auth.database.getProjectById(allocator, pool, project_id) catch {
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
    const datasource_id = auth.database.createDatasource(allocator, pool, project_id, data.name, data.type, data.config) catch |err| {
        std.log.err("Failed to create datasource: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create datasource\"}";
        res.json(allocator, error_json);
        return;
    };
    defer allocator.free(datasource_id);

    const response_json = std.fmt.allocPrint(allocator,
        \\{{"success": true, "message": "Datasource created", "datasource_id": "{s}"}}
    , .{datasource_id}) catch {
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create response\"}";
        res.json(allocator, error_json);
        return;
    };

    res.status_code = 201;
    res.json(allocator, response_json);
}

fn getDatasourceHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, pool: *pg.Pool) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, pool) orelse return;

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
    const project = auth.database.getProjectById(allocator, pool, project_id) catch {
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
    const datasource = auth.database.getDatasourceById(allocator, pool, datasource_id) catch |err| {
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

    const response_json = std.fmt.allocPrint(allocator,
        \\{{"id":"{s}","project_id":"{s}","name":"{s}","type":"{s}","config":{s},"is_active":{},"created_at":"{s}"}}
    , .{ datasource.id, datasource.project_id, datasource.name, datasource.type, datasource.config, datasource.is_active, datasource.created_at }) catch {
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create response\"}";
        res.json(allocator, error_json);
        return;
    };

    res.json(allocator, response_json);
}

fn updateDatasourceHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, pool: *pg.Pool) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, pool) orelse return;

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
    const project = auth.database.getProjectById(allocator, pool, project_id) catch {
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
    auth.database.updateDatasource(pool, datasource_id, data.name, data.type, data.config) catch |err| {
        std.log.err("Failed to update datasource: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to update datasource\"}";
        res.json(allocator, error_json);
        return;
    };

    const response_json = "{\"success\": true, \"message\": \"Datasource updated\"}";
    res.json(allocator, response_json);
}

fn deleteDatasourceHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, pool: *pg.Pool) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get user from session
    const user = getUserFromSession(req, res, allocator, pool) orelse return;

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
    const project = auth.database.getProjectById(allocator, pool, project_id) catch {
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
    auth.database.deleteDatasource(pool, datasource_id) catch |err| {
        std.log.err("Failed to delete datasource: {}", .{err});
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to delete datasource\"}";
        res.json(allocator, error_json);
        return;
    };

    const response_json = "{\"success\": true, \"message\": \"Datasource deleted\"}";
    res.json(allocator, response_json);
}

fn getUserFromSession(req: *Request, res: *Response, allocator: std.mem.Allocator, pool: *pg.Pool) ?auth.User {
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
    const user = auth.database.validateSession(allocator, pool, session_token) catch |err| switch (err) {
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

fn profileHandler(req: *Request, res: *Response, allocator: std.mem.Allocator, pool: *pg.Pool) void {
    // Add CORS headers
    setCorsHeaders(res, req);

    // Get session token from cookie
    const cookie_header = req.headers.get("Cookie") orelse req.headers.get("cookie");
    const cookie_header_unwrapped = cookie_header orelse {
        res.status_code = 401;
        const error_json = "{\"error\": \"No session found\"}";
        res.json(allocator, error_json);
        return;
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
        return;
    };

    // Validate session and get user
    const user = auth.validateSession(allocator, pool, session_token) catch |err| {
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

    // Return user profile
    const response_json = std.fmt.allocPrint(allocator,
        \\{{"success": true, "user": {{ "id": "{s}", "username": "{s}", "created_at": "{s}" }}}}
    , .{ user.id, user.username, user.created_at }) catch {
        res.status_code = 500;
        const error_json = "{\"error\": \"Failed to create response\"}";
        res.json(allocator, error_json);
        return;
    };

    res.json(allocator, response_json);
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

pub fn main() !void {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    std.debug.print("Starting server...\n", .{});

    // Initialize rate limiter
    RateLimiter.init();
    defer RateLimiter.deinit();

    // Load environment variables
    var env_map = try loadEnv(allocator);
    defer {
        var it = env_map.iterator();
        while (it.next()) |entry| {
            allocator.free(entry.key_ptr.*);
            allocator.free(entry.value_ptr.*);
        }
        env_map.deinit();
    }

    const database_url = env_map.get("DATABASE_URL") orelse {
        std.log.err("DATABASE_URL not found in environment", .{});
        return error.MissingDatabaseUrl;
    };

    // Connect to database
    const uri = try std.Uri.parse(database_url);
    var pool = try pg.Pool.initUri(allocator, uri, .{
        .size = 5,
        .timeout = 10_000,
    });
    defer pool.deinit();

    std.log.info("Connected to PostgreSQL successfully!", .{});

    // Initialize database schema
    std.log.info("Initializing database schema...", .{});

    // Temporarily skip schema execution to avoid errors
    // if (std.fs.cwd().openFile("schema.sql", .{})) |file| {
    //     defer file.close();
    //     const schema_content = try file.readToEndAlloc(std.heap.page_allocator, 1024 * 1024);
    //     defer std.heap.page_allocator.free(schema_content);

    //     // Execute schema
    //     _ = pool.exec(schema_content, .{}) catch |err| {
    //         std.log.err("Error executing schema: {}", .{err});
    //         return err;
    //     };
    //     std.log.info("Database schema initialized successfully!", .{});
    // } else |err| switch (err) {
    //     error.FileNotFound => {
    //         std.log.warn("schema.sql not found, skipping initialization", .{});
    //     },
    //     else => return err,
    // }

    // Check if we're in production mode
    const is_prod_result = blk: {
        const result = std.process.getEnvVarOwned(allocator, "NODE_ENV") catch |err| {
            if (err == error.EnvironmentVariableNotFound) {
                break :blk try allocator.dupe(u8, "");
            }
            return err;
        };
        break :blk result;
    };
    defer allocator.free(is_prod_result);
    const production = std.mem.eql(u8, is_prod_result, "production");

    const port: u16 = if (production) 3000 else 8080;

    var router = Router.init(allocator);

    // Add authentication routes (available in both dev and prod)
    router.getSimple(allocator, "/api/health", healthHandler);
    router.options(allocator, "/api/auth/register", corsHandler);
    router.post(allocator, "/api/auth/register", registerHandler);
    router.options(allocator, "/api/auth/login", corsHandler);
    router.post(allocator, "/api/auth/login", loginHandler);
    router.options(allocator, "/api/auth/logout", corsHandler);
    router.post(allocator, "/api/auth/logout", logoutHandler);
    router.options(allocator, "/api/auth/profile", corsHandler);
    router.get(allocator, "/api/auth/profile", profileHandler);

    // Add project routes
    router.options(allocator, "/api/projects", corsHandler);
    router.get(allocator, "/api/projects", getProjectsHandler);
    router.post(allocator, "/api/projects", createProjectHandler);
    router.options(allocator, "/api/projects/*", corsHandler);
    router.get(allocator, "/api/projects/*", getProjectHandler);
    router.put(allocator, "/api/projects/*", updateProjectHandler);
    router.delete(allocator, "/api/projects/*", deleteProjectHandler);

    // Add datasource routes
    router.options(allocator, "/api/projects/*/datasources", corsHandler);
    router.get(allocator, "/api/projects/*/datasources", getDatasourcesHandler);
    router.post(allocator, "/api/projects/*/datasources", createDatasourceHandler);
    router.options(allocator, "/api/projects/*/datasources/*", corsHandler);
    router.get(allocator, "/api/projects/*/datasources/*", getDatasourceHandler);
    router.put(allocator, "/api/projects/*/datasources/*", updateDatasourceHandler);
    router.delete(allocator, "/api/projects/*/datasources/*", deleteDatasourceHandler);

    if (production) {
        // In production, serve the SPA
        router.getSimple(allocator, "/", spaHandler);
        router.getSimple(allocator, "/api", apiHandler);
        router.websocket(allocator, "/ws", websocketHandler);
    } else {
        // In development, use the original handlers
        router.getSimple(allocator, "/", homeHandler);
        router.getSimple(allocator, "/api", apiHandler);
        router.websocket(allocator, "/ws", websocketHandler);
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
        std.debug.print("Accepted connection\n", .{});

        var buffer: [4096]u8 = undefined;
        const bytes_read = try connection.stream.read(&buffer);
        std.debug.print("Read {d} bytes\n", .{bytes_read});

        if (bytes_read == 0) {
            connection.stream.close();
            continue;
        }

        const request_data = buffer[0..bytes_read];
        std.debug.print("Raw request:\n{s}\n", .{request_data});
        var request = parseRequest(request_data, allocator) catch {
            print("Failed to parse request\n", .{});
            connection.stream.close();
            continue;
        };
        defer request.headers.deinit();

        // Debug: Print request info
        std.debug.print("Request: {s} {s}\n", .{ request.method, request.path });

        if (isWebSocketUpgrade(&request)) {
            const websocket_key = request.headers.get("Sec-WebSocket-Key") orelse {
                connection.stream.close();
                continue;
            };

            const accept_key = try generateWebSocketAcceptKey(websocket_key, allocator);
            defer allocator.free(accept_key);

            var response = Response.init(allocator);
            defer response.headers.deinit();
            response.status_code = 101;
            response.headers.put("Upgrade", "websocket") catch {};
            response.headers.put("Connection", "Upgrade") catch {};
            response.headers.put("Sec-WebSocket-Accept", accept_key) catch {};

            const response_str = try response.toString(allocator);
            defer allocator.free(response_str);

            _ = try connection.stream.writeAll(response_str);

            var ws_connection = WebSocketConnection{
                .stream = connection.stream,
                .allocator = allocator,
            };

            if (!router.handleWebSocket(request.path, &ws_connection, allocator)) {
                ws_connection.close() catch {};
            }
        } else {
            defer connection.stream.close();

            var response = Response.init(allocator);
            defer response.headers.deinit();

            const handle_result = router.handle(request.method, request.path, &request, &response, allocator, pool);
            std.debug.print("Router handle result: {}\n", .{handle_result});
            if (!handle_result) {
                print("Route not found, checking fallbacks\n", .{});
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
                    std.debug.print("Setting 404 response\n", .{});
                    response.status_code = 404;
                    const not_found = "{\"error\": \"Not Found\"}";
                    response.json(allocator, not_found);
                }
            }

            const response_str = try response.toString(allocator);
            defer allocator.free(response_str);

            _ = try connection.stream.writeAll(response_str);

            allocator.free(request.method);
            allocator.free(request.path);
            allocator.free(request.body);
            if (response.body.len > 0) {
                allocator.free(response.body);
            }
        }
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
