const std = @import("std");
const zlay_db = @import("zlay-db");
const types = @import("types.zig");

const User = types.User;
const Session = types.Session;
const Project = types.Project;
const Datasource = types.Datasource;
const Client = types.Client;
const Domain = types.Domain;
const AuthError = types.AuthError;

/// Register a new user for a specific client
pub fn registerUser(
    allocator: std.mem.Allocator,
    db: *zlay_db.Database,
    client_id: []const u8,
    username: []const u8,
    password_hash: []const u8,
) !User {
    // Escape strings to prevent SQL injection
    const escaped_username = try escapeSqlString(allocator, username);
    defer allocator.free(escaped_username);
    const escaped_password_hash = try escapeSqlString(allocator, password_hash);
    defer allocator.free(escaped_password_hash);

    // Insert user with manual SQL construction (workaround for zlay-db parameter binding issue)
    var sql_buf: [2048]u8 = undefined;
    const sql = std.fmt.bufPrint(&sql_buf,
        \\INSERT INTO users (client_id, username, password_hash)
        \\VALUES ('{s}', '{s}', '{s}')
        \\ON CONFLICT (client_id, username) DO NOTHING
        \\RETURNING id::text, client_id::text, username, password_hash, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at, is_active
    , .{ client_id, escaped_username, escaped_password_hash }) catch {
        std.log.err("Failed to format SQL", .{});
        return AuthError.DatabaseError;
    };

    const result = db.query(sql, .{}) catch |err| {
        std.log.err("User creation failed: {}", .{err});
        return AuthError.DatabaseError;
    };
    defer result.deinit();

    if (result.rows.len == 0) {
        std.log.info("User already exists", .{});
        return AuthError.UserAlreadyExists;
    }
    const row = result.rows[0];

    return User{
        .id = allocator.dupe(u8, row.values[0].text) catch return AuthError.DatabaseError,
        .client_id = allocator.dupe(u8, row.values[1].text) catch return AuthError.DatabaseError,
        .username = allocator.dupe(u8, row.values[2].text) catch return AuthError.DatabaseError,
        .password_hash = allocator.dupe(u8, row.values[3].text) catch return AuthError.DatabaseError,
        .created_at = allocator.dupe(u8, row.values[4].text) catch return AuthError.DatabaseError,
        .is_active = row.values[5].asBoolean() orelse false,
    };
}

/// Validate that a client exists and is active
pub fn validateClient(
    db: *zlay_db.Database,
    client_id: []const u8,
) !void {
    // Check if client exists and is active
    const result = db.query("SELECT id FROM clients WHERE id = $1 AND is_active = true", .{client_id}) catch return AuthError.DatabaseError;
    defer result.deinit();

    if (result.rows.len == 0) return AuthError.InvalidClient;
}

/// Get client by ID
pub fn getClientById(
    allocator: std.mem.Allocator,
    db: *zlay_db.Database,
    client_id: []const u8,
) !Client {
    const result = db.query("SELECT id::text, name, slug, ai_api_key, ai_api_url, ai_api_model, is_active::boolean, to_char(created_at, 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') as created_at FROM clients WHERE id = $1", .{client_id}) catch return AuthError.DatabaseError;
    defer result.deinit();

    if (result.rows.len == 0) return AuthError.InvalidClient;
    const row = result.rows[0];

    return Client{
        .id = try allocator.dupe(u8, row.values[0].text),
        .name = try allocator.dupe(u8, row.values[1].text),
        .slug = try allocator.dupe(u8, row.values[2].text),
        .ai_api_key = if (row.values[3].asText()) |text| if (text.len > 0) try allocator.dupe(u8, text) else null else null,
        .ai_api_url = if (row.values[4].asText()) |text| if (text.len > 0) try allocator.dupe(u8, text) else null else null,
        .ai_api_model = if (row.values[5].asText()) |text| if (text.len > 0) try allocator.dupe(u8, text) else null else null,
        .is_active = row.values[6].asBoolean() orelse false,
        .created_at = try allocator.dupe(u8, row.values[7].text),
    };
}

/// Get user by client_id and username
pub fn getUserByCredentials(
    allocator: std.mem.Allocator,
    db: *zlay_db.Database,
    client_id: []const u8,
    username: []const u8,
) !User {
    // Special handling for root user - can login from any client
    if (std.mem.eql(u8, username, "root")) {
        var sql_buf: [1024]u8 = undefined;
        const sql = std.fmt.bufPrint(&sql_buf,
            \\SELECT id::text, client_id::text, username, password_hash, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at, is_active
            \\FROM users WHERE username = 'root' AND is_active = true LIMIT 1
        , .{}) catch return AuthError.DatabaseError;

        const user_result = db.query(sql, .{}) catch return AuthError.DatabaseError;
        defer user_result.deinit();

        if (user_result.rows.len == 0) return AuthError.UserNotFound;
        const row = user_result.rows[0];

        return User{
            .id = allocator.dupe(u8, row.values[0].text) catch return AuthError.DatabaseError,
            .client_id = allocator.dupe(u8, row.values[1].text) catch return AuthError.DatabaseError,
            .username = allocator.dupe(u8, row.values[2].text) catch return AuthError.DatabaseError,
            .password_hash = allocator.dupe(u8, row.values[3].text) catch return AuthError.DatabaseError,
            .created_at = allocator.dupe(u8, row.values[4].text) catch return AuthError.DatabaseError,
            .is_active = row.values[5].asBoolean() orelse false,
        };
    }

    // Regular user - must match client_id
    var sql_buf: [1024]u8 = undefined;
    const sql = std.fmt.bufPrint(&sql_buf,
        \\SELECT id::text, client_id::text, username, password_hash, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at, is_active
        \\FROM users WHERE client_id = '{s}' AND username = '{s}' AND is_active = true
    , .{ client_id, username }) catch return AuthError.DatabaseError;

    const user_result = db.query(sql, .{}) catch return AuthError.DatabaseError;
    defer user_result.deinit();

    if (user_result.rows.len == 0) return AuthError.UserNotFound;
    const row = user_result.rows[0];

    return User{
        .id = allocator.dupe(u8, row.values[0].text) catch return AuthError.DatabaseError,
        .client_id = allocator.dupe(u8, row.values[1].text) catch return AuthError.DatabaseError,
        .username = allocator.dupe(u8, row.values[2].text) catch return AuthError.DatabaseError,
        .password_hash = allocator.dupe(u8, row.values[3].text) catch return AuthError.DatabaseError,
        .created_at = allocator.dupe(u8, row.values[4].text) catch return AuthError.DatabaseError,
        .is_active = row.values[5].asBoolean() orelse false,
    };
}

/// Create session in database
pub fn createSession(
    allocator: std.mem.Allocator,
    db: *zlay_db.Database,
    client_id: []const u8,
    user_id: []const u8,
    token_hash: []const u8,
    expires_at: i64,
) ![]const u8 {
    // Escape token_hash to prevent SQL injection
    const escaped_token_hash = try escapeSqlString(allocator, token_hash);
    defer allocator.free(escaped_token_hash);

    // Use manual SQL construction (workaround for zlay-db parameter binding issue)
    var sql_buf: [1024]u8 = undefined;
    const sql = std.fmt.bufPrint(&sql_buf,
        \\INSERT INTO sessions (client_id, user_id, token_hash, expires_at)
        \\VALUES ('{s}', '{s}', '{s}', to_timestamp({}))
        \\RETURNING id::text
    , .{ client_id, user_id, escaped_token_hash, expires_at }) catch return AuthError.DatabaseError;

    const session_result = db.query(sql, .{}) catch |err| {
        std.log.err("Session creation failed: {}", .{err});
        return AuthError.DatabaseError;
    };
    defer session_result.deinit();

    if (session_result.rows.len == 0) return AuthError.DatabaseError;
    return allocator.dupe(u8, session_result.rows[0].values[0].text);
}

/// Validate session token and return user
pub fn validateSession(
    allocator: std.mem.Allocator,
    db: *zlay_db.Database,
    token_hash: []const u8,
) !User {
    // First, get the user_id from the session (manual SQL construction)
    var sql_buf: [512]u8 = undefined;
    const session_sql = std.fmt.bufPrint(&sql_buf,
        \\SELECT user_id::text FROM sessions WHERE token_hash = '{s}' AND expires_at > NOW() LIMIT 1
    , .{token_hash}) catch return AuthError.DatabaseError;

    const session_result = db.query(session_sql, .{}) catch |err| {
        std.log.err("Session query failed: {}", .{err});
        return AuthError.DatabaseError;
    };
    defer session_result.deinit();

    if (session_result.rows.len == 0) return AuthError.InvalidToken;
    const user_id = session_result.rows[0].values[0].text;

    // Then, get the user details (manual SQL construction)
    var user_sql_buf: [1024]u8 = undefined;
    const user_sql = std.fmt.bufPrint(&user_sql_buf,
        \\SELECT id::text, client_id::text, username, password_hash, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at, is_active
        \\FROM users WHERE id = '{s}' AND is_active = true LIMIT 1
    , .{user_id}) catch return AuthError.DatabaseError;

    const user_result = db.query(user_sql, .{}) catch |err| {
        std.log.err("User query failed: {}", .{err});
        return AuthError.DatabaseError;
    };
    defer user_result.deinit();

    if (user_result.rows.len == 0) return AuthError.InvalidToken;
    const row = user_result.rows[0];

    // Batch allocate all strings at once to reduce allocation overhead
    const id = allocator.dupe(u8, row.values[0].text) catch return AuthError.DatabaseError;
    errdefer allocator.free(id);
    const client_id = allocator.dupe(u8, row.values[1].text) catch return AuthError.DatabaseError;
    errdefer allocator.free(client_id);
    const username = allocator.dupe(u8, row.values[2].text) catch return AuthError.DatabaseError;
    errdefer allocator.free(username);
    const password_hash = allocator.dupe(u8, row.values[3].text) catch return AuthError.DatabaseError;
    errdefer allocator.free(password_hash);
    const created_at = allocator.dupe(u8, row.values[4].text) catch return AuthError.DatabaseError;
    errdefer allocator.free(created_at);

    return User{
        .id = id,
        .client_id = client_id,
        .username = username,
        .password_hash = password_hash,
        .created_at = created_at,
        .is_active = std.mem.eql(u8, row.values[5].text, "t"),
    };
}

/// Logout user by invalidating session
pub fn deleteSession(
    db: *zlay_db.Database,
    token_hash: []const u8,
) !void {
    _ = db.exec("DELETE FROM sessions WHERE token_hash = $1", .{token_hash}) catch return AuthError.DatabaseError;
}

/// Clean up expired sessions
pub fn cleanupExpiredSessions(db: *zlay_db.Database) !void {
    const current_time = std.time.timestamp();
    _ = db.exec("DELETE FROM sessions WHERE expires_at <= to_timestamp($1)", .{current_time}) catch return AuthError.DatabaseError;
}

/// Create a new project
pub fn createProject(
    allocator: std.mem.Allocator,
    db: *zlay_db.Database,
    user_id: []const u8,
    name: []const u8,
    description: []const u8,
) ![]const u8 {
    // Escape strings to prevent SQL injection
    const escaped_name = try escapeSqlString(allocator, name);
    defer allocator.free(escaped_name);
    const escaped_description = try escapeSqlString(allocator, description);
    defer allocator.free(escaped_description);

    // Use manual SQL construction (workaround for zlay-db parameter binding issue)
    var sql_buf: [1024]u8 = undefined;
    const sql = std.fmt.bufPrint(&sql_buf,
        \\INSERT INTO projects (user_id, name, description)
        \\VALUES ('{s}', '{s}', '{s}')
        \\RETURNING id::text
    , .{ user_id, escaped_name, escaped_description }) catch return AuthError.DatabaseError;

    const result = db.query(sql, .{}) catch |err| {
        std.log.err("Project creation failed: {}", .{err});
        return AuthError.DatabaseError;
    };
    defer result.deinit();

    if (result.rows.len == 0) return AuthError.DatabaseError;
    return allocator.dupe(u8, result.rows[0].values[0].text);
}

/// Get projects by user ID
pub fn getProjectsByUser(
    allocator: std.mem.Allocator,
    db: *zlay_db.Database,
    user_id: []const u8,
) ![]Project {
    // Use manual SQL construction (workaround for zlay-db parameter binding issue)
    var sql_buf: [512]u8 = undefined;
    const sql = std.fmt.bufPrint(&sql_buf,
        \\SELECT id::text, user_id::text, name, description, is_active::boolean, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at
        \\FROM projects WHERE user_id = '{s}' AND is_active = true ORDER BY created_at DESC
    , .{user_id}) catch return AuthError.DatabaseError;

    const result = db.query(sql, .{}) catch return AuthError.DatabaseError;
    defer result.deinit();

    var projects = try std.ArrayList(Project).initCapacity(allocator, 0);
    defer projects.deinit(allocator);

    for (result.rows) |row| {
        try projects.append(allocator, Project{
            .id = try allocator.dupe(u8, row.values[0].text),
            .user_id = try allocator.dupe(u8, row.values[1].text),
            .name = try allocator.dupe(u8, row.values[2].text),
            .description = try allocator.dupe(u8, row.values[3].text),
            .is_active = row.values[4].asBoolean() orelse false,
            .created_at = try allocator.dupe(u8, row.values[5].text),
        });
    }

    return projects.toOwnedSlice(allocator);
}

/// Get project by ID
pub fn getProjectById(
    allocator: std.mem.Allocator,
    db: *zlay_db.Database,
    project_id: []const u8,
) !Project {
    // Use manual SQL construction (workaround for zlay-db parameter binding issue)
    var sql_buf: [512]u8 = undefined;
    const sql = std.fmt.bufPrint(&sql_buf,
        \\SELECT id::text, user_id::text, name, description, is_active::boolean, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at
        \\FROM projects WHERE id = '{s}' AND is_active = true
    , .{project_id}) catch return AuthError.DatabaseError;

    const result = db.query(sql, .{}) catch return AuthError.DatabaseError;
    defer result.deinit();

    if (result.rows.len == 0) return AuthError.DatabaseError;
    const row = result.rows[0];
    return Project{
        .id = try allocator.dupe(u8, row.values[0].text),
        .user_id = try allocator.dupe(u8, row.values[1].text),
        .name = try allocator.dupe(u8, row.values[2].text),
        .description = try allocator.dupe(u8, row.values[3].text),
        .is_active = row.values[4].boolean,
        .created_at = try allocator.dupe(u8, row.values[5].text),
    };
}

/// Update project
pub fn updateProject(
    db: *zlay_db.Database,
    project_id: []const u8,
    name: []const u8,
    description: []const u8,
) !void {
    // Use manual SQL construction (workaround for zlay-db parameter binding issue)
    var sql_buf: [512]u8 = undefined;
    const sql = std.fmt.bufPrint(&sql_buf,
        \\UPDATE projects SET name = '{s}', description = '{s}' WHERE id = '{s}'
    , .{ name, description, project_id }) catch return AuthError.DatabaseError;

    _ = db.exec(sql, .{}) catch return AuthError.DatabaseError;
}

/// Delete project (soft delete)
pub fn deleteProject(db: *zlay_db.Database, project_id: []const u8) !void {
    // Use manual SQL construction (workaround for zlay-db parameter binding issue)
    var sql_buf: [256]u8 = undefined;
    const sql = std.fmt.bufPrint(&sql_buf,
        \\UPDATE projects SET is_active = false WHERE id = '{s}'
    , .{project_id}) catch return AuthError.DatabaseError;

    _ = db.exec(sql, .{}) catch return AuthError.DatabaseError;
}

/// Create a new datasource
pub fn createDatasource(
    allocator: std.mem.Allocator,
    db: *zlay_db.Database,
    project_id: []const u8,
    name: []const u8,
    ds_type: []const u8,
    config: []const u8,
) ![]const u8 {
    std.log.info("database.createDatasource: project_id={s}, name={s}", .{ project_id, name });

    // Insert datasource and return ID in single query
    const result = db.query("INSERT INTO datasources (project_id, name, type, config) VALUES ($1, $2, $3, $4::jsonb) RETURNING id::text", .{ project_id, name, ds_type, config }) catch |err| {
        std.log.err("Datasource creation failed: {}", .{err});
        return AuthError.DatabaseError;
    };
    defer result.deinit();

    if (result.rows.len == 0) return AuthError.DatabaseError;
    return allocator.dupe(u8, result.rows[0].values[0].text);
}

/// Get datasources by project ID
pub fn getDatasourcesByProject(
    allocator: std.mem.Allocator,
    db: *zlay_db.Database,
    project_id: []const u8,
) ![]Datasource {
    const result = db.query("SELECT id::text, project_id::text, name, type, config::text, is_active::boolean, to_char(created_at, 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') as created_at FROM datasources WHERE project_id = $1 AND is_active = true ORDER BY created_at DESC", .{project_id}) catch return AuthError.DatabaseError;
    defer result.deinit();

    var datasources = try std.ArrayList(Datasource).initCapacity(allocator, 0);
    defer datasources.deinit(allocator);

    for (result.rows) |row| {
        try datasources.append(allocator, Datasource{
            .id = try allocator.dupe(u8, row.values[0].text),
            .project_id = try allocator.dupe(u8, row.values[1].text),
            .name = try allocator.dupe(u8, row.values[2].text),
            .type = try allocator.dupe(u8, row.values[3].text),
            .config = try allocator.dupe(u8, row.values[4].text),
            .is_active = row.values[5].asBoolean() orelse false,
            .created_at = try allocator.dupe(u8, row.values[6].text),
        });
    }

    return datasources.toOwnedSlice(allocator);
}

/// Get datasource by ID
pub fn getDatasourceById(
    allocator: std.mem.Allocator,
    db: *zlay_db.Database,
    datasource_id: []const u8,
) !Datasource {
    const result = db.query("SELECT id::text, project_id::text, name, type, config::text, is_active::boolean, to_char(created_at, 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') as created_at FROM datasources WHERE id = $1 AND is_active = true", .{datasource_id}) catch return AuthError.DatabaseError;
    defer result.deinit();

    if (result.rows.len == 0) return AuthError.DatabaseError;
    const row = result.rows[0];
    return Datasource{
        .id = try allocator.dupe(u8, row.values[0].text),
        .project_id = try allocator.dupe(u8, row.values[1].text),
        .name = try allocator.dupe(u8, row.values[2].text),
        .type = try allocator.dupe(u8, row.values[3].text),
        .config = try allocator.dupe(u8, row.values[4].text),
        .is_active = row.values[5].asBoolean() orelse false,
        .created_at = try allocator.dupe(u8, row.values[6].text),
    };
}

/// Update datasource
pub fn updateDatasource(
    db: *zlay_db.Database,
    datasource_id: []const u8,
    name: []const u8,
    ds_type: []const u8,
    config: []const u8,
) !void {
    _ = db.exec("UPDATE datasources SET name = $1, type = $2, config = $3::jsonb WHERE id = $4", .{ name, ds_type, config, datasource_id }) catch return AuthError.DatabaseError;
}

/// Delete datasource (soft delete)
pub fn deleteDatasource(db: *zlay_db.Database, datasource_id: []const u8) !void {
    _ = db.exec("UPDATE datasources SET is_active = false WHERE id = $1", .{datasource_id}) catch return AuthError.DatabaseError;
}

/// Create a new client
fn escapeSqlString(allocator: std.mem.Allocator, input: []const u8) ![]const u8 {
    // Escape single quotes by doubling them for PostgreSQL
    var escaped = try std.ArrayList(u8).initCapacity(allocator, input.len * 2);
    defer escaped.deinit(allocator);
    for (input) |char| {
        if (char == '\'') {
            try escaped.append(allocator, '\'');
        }
        try escaped.append(allocator, char);
    }
    return try escaped.toOwnedSlice(allocator);
}

pub fn createClient(
    allocator: std.mem.Allocator,
    db: *zlay_db.Database,
    name: []const u8,
    slug: []const u8,
    ai_api_key: ?[]const u8,
    ai_api_url: ?[]const u8,
    ai_api_model: ?[]const u8,
) ![]const u8 {
    _ = ai_api_key;
    _ = ai_api_url;
    _ = ai_api_model;

    // Escape the name and slug to prevent SQL injection
    const escaped_name = try escapeSqlString(allocator, name);
    defer allocator.free(escaped_name);
    const escaped_slug = try escapeSqlString(allocator, slug);
    defer allocator.free(escaped_slug);

    // Use manual SQL construction (workaround for zlay-db parameter binding issue)
    var sql_buf: [1024]u8 = undefined; // Increased buffer size
    const sql = std.fmt.bufPrint(&sql_buf,
        \\INSERT INTO clients (name, slug, ai_api_key, ai_api_url, ai_api_model)
        \\VALUES ('{s}', '{s}', NULL, NULL, NULL)
        \\RETURNING id::text
    , .{ escaped_name, escaped_slug }) catch return AuthError.DatabaseError;

    const result = db.query(sql, .{}) catch return AuthError.DatabaseError;
    defer result.deinit();

    if (result.rows.len == 0) return AuthError.DatabaseError;
    return allocator.dupe(u8, result.rows[0].values[0].text);
}

/// Get all clients
pub fn getAllClients(allocator: std.mem.Allocator, db: *zlay_db.Database) ![]Client {
    const result = db.query("SELECT id::text, name, slug, ai_api_key, ai_api_url, ai_api_model, is_active::boolean, to_char(created_at, 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') as created_at FROM clients ORDER BY created_at DESC", .{}) catch return AuthError.DatabaseError;
    defer result.deinit();

    var clients = try std.ArrayList(Client).initCapacity(allocator, result.rows.len);
    defer clients.deinit(allocator);

    for (result.rows) |row| {
        try clients.append(allocator, Client{
            .id = try allocator.dupe(u8, row.values[0].text),
            .name = try allocator.dupe(u8, row.values[1].text),
            .slug = try allocator.dupe(u8, row.values[2].text),
            .ai_api_key = if (row.values[3].asText()) |text| if (text.len > 0) try allocator.dupe(u8, text) else null else null,
            .ai_api_url = if (row.values[4].asText()) |text| if (text.len > 0) try allocator.dupe(u8, text) else null else null,
            .ai_api_model = if (row.values[5].asText()) |text| if (text.len > 0) try allocator.dupe(u8, text) else null else null,
            .is_active = row.values[6].asBoolean() orelse false,
            .created_at = try allocator.dupe(u8, row.values[7].text),
        });
    }

    return clients.toOwnedSlice(allocator);
}

/// Update client
pub fn updateClient(
    db: *zlay_db.Database,
    client_id: []const u8,
    name: []const u8,
    slug: []const u8,
    ai_api_key: ?[]const u8,
    ai_api_url: ?[]const u8,
    ai_api_model: ?[]const u8,
    is_active: bool,
) !void {
    // Escape name and slug to prevent SQL injection
    var arena = std.heap.ArenaAllocator.init(std.heap.page_allocator);
    defer arena.deinit();
    const allocator = arena.allocator();

    const escaped_name = try escapeSqlString(allocator, name);
    const escaped_slug = try escapeSqlString(allocator, slug);

    const escaped_ai_api_key = if (ai_api_key) |key| try escapeSqlString(allocator, key) else null;
    const escaped_ai_api_url = if (ai_api_url) |url| try escapeSqlString(allocator, url) else null;
    const escaped_ai_api_model = if (ai_api_model) |model| try escapeSqlString(allocator, model) else null;

    // Use manual SQL construction (workaround for zlay-db parameter binding issue)
    var sql_buf: [2048]u8 = undefined;
    const sql = std.fmt.bufPrint(&sql_buf,
        \\UPDATE clients SET name = '{s}', slug = '{s}', ai_api_key = {s}, ai_api_url = {s}, ai_api_model = {s}, is_active = {}
        \\WHERE id = '{s}'
    , .{ escaped_name, escaped_slug, if (escaped_ai_api_key) |key| std.fmt.allocPrint(allocator, "'{s}'", .{key}) catch "NULL" else "NULL", if (escaped_ai_api_url) |url| std.fmt.allocPrint(allocator, "'{s}'", .{url}) catch "NULL" else "NULL", if (escaped_ai_api_model) |model| std.fmt.allocPrint(allocator, "'{s}'", .{model}) catch "NULL" else "NULL", is_active, client_id }) catch return AuthError.DatabaseError;

    _ = db.exec(sql, .{}) catch return AuthError.DatabaseError;
}

/// Delete client (hard delete)
pub fn deleteClient(db: *zlay_db.Database, client_id: []const u8) !void {
    // Delete associated sessions first
    _ = db.exec("DELETE FROM sessions WHERE client_id = $1", .{client_id}) catch return AuthError.DatabaseError;
    // Delete associated users (this will cascade to projects and datasources)
    _ = db.exec("DELETE FROM users WHERE client_id = $1", .{client_id}) catch return AuthError.DatabaseError;
    // Delete associated domains
    _ = db.exec("DELETE FROM domains WHERE client_id = $1", .{client_id}) catch return AuthError.DatabaseError;
    // Finally delete the client
    _ = db.exec("DELETE FROM clients WHERE id = $1", .{client_id}) catch return AuthError.DatabaseError;
}

/// Add domain to client
pub fn addDomainToClient(
    allocator: std.mem.Allocator,
    db: *zlay_db.Database,
    client_id: []const u8,
    domain: []const u8,
) ![]const u8 {
    // Escape domain to prevent SQL injection
    const escaped_domain = try escapeSqlString(allocator, domain);
    defer allocator.free(escaped_domain);

    // Use manual SQL construction (workaround for zlay-db parameter binding issue)
    var sql_buf: [512]u8 = undefined;
    const sql = std.fmt.bufPrint(&sql_buf,
        \\INSERT INTO domains (client_id, domain)
        \\VALUES ('{s}', '{s}')
        \\RETURNING id::text
    , .{ client_id, escaped_domain }) catch return AuthError.DatabaseError;

    const result = db.query(sql, .{}) catch return AuthError.DatabaseError;
    defer result.deinit();

    if (result.rows.len == 0) return AuthError.DatabaseError;
    return allocator.dupe(u8, result.rows[0].values[0].text);
}

/// Get domains by client
pub fn getDomainsByClient(allocator: std.mem.Allocator, db: *zlay_db.Database, client_id: []const u8) ![]Domain {
    const result = db.query("SELECT id::text, client_id::text, domain, is_active::boolean, to_char(created_at, 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') as created_at FROM domains WHERE client_id = $1 ORDER BY created_at DESC", .{client_id}) catch return AuthError.DatabaseError;
    defer result.deinit();

    var domains = try std.ArrayList(Domain).initCapacity(allocator, result.rows.len);
    defer domains.deinit(allocator);

    for (result.rows) |row| {
        try domains.append(allocator, Domain{
            .id = try allocator.dupe(u8, row.values[0].text),
            .client_id = try allocator.dupe(u8, row.values[1].text),
            .domain = try allocator.dupe(u8, row.values[2].text),
            .is_active = row.values[3].asBoolean() orelse false,
            .created_at = try allocator.dupe(u8, row.values[4].text),
        });
    }

    return domains.toOwnedSlice(allocator);
}

/// Remove domain from client
pub fn removeDomain(db: *zlay_db.Database, domain_id: []const u8) !void {
    // Use manual SQL construction (workaround for zlay-db parameter binding issue)
    var sql_buf: [256]u8 = undefined;
    const sql = std.fmt.bufPrint(&sql_buf,
        \\DELETE FROM domains WHERE id = '{s}'
    , .{domain_id}) catch return AuthError.DatabaseError;

    _ = db.exec(sql, .{}) catch return AuthError.DatabaseError;
}

/// Update domain status
pub fn updateDomain(db: *zlay_db.Database, domain_id: []const u8, is_active: bool) !void {
    std.log.info("Updating domain {s} to active: {}", .{ domain_id, is_active });
    // Use manual SQL construction (workaround for zlay-db parameter binding issue)
    var sql_buf: [256]u8 = undefined;
    const sql = std.fmt.bufPrint(&sql_buf,
        \\UPDATE domains SET is_active = {} WHERE id = '{s}'
    , .{ is_active, domain_id }) catch return AuthError.DatabaseError;

    _ = db.exec(sql, .{}) catch return AuthError.DatabaseError;
}
