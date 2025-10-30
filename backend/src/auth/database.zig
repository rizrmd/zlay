const std = @import("std");
const zlay_db = @import("zlay-db");
const types = @import("types.zig");

const User = types.User;
const Session = types.Session;
const Project = types.Project;
const Datasource = types.Datasource;
const AuthError = types.AuthError;

/// Register a new user for a specific client
pub fn registerUser(
    allocator: std.mem.Allocator,
    db: *zlay_db.Database,
    client_id: []const u8,
    username: []const u8,
    password_hash: []const u8,
) !User {
    // Insert user with manual SQL construction (workaround for zlay-db parameter binding issue)
    var sql_buf: [2048]u8 = undefined;
    const sql = std.fmt.bufPrint(&sql_buf,
        \\INSERT INTO users (client_id, username, password_hash)
        \\VALUES ('{s}', '{s}', '{s}')
        \\ON CONFLICT (client_id, username) DO NOTHING
        \\RETURNING id::text, client_id::text, username, password_hash, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at, is_active
    , .{ client_id, username, password_hash }) catch {
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

/// Get user by client_id and username
pub fn getUserByCredentials(
    allocator: std.mem.Allocator,
    db: *zlay_db.Database,
    client_id: []const u8,
    username: []const u8,
) !User {
    // Use manual SQL construction (workaround for zlay-db parameter binding issue)
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
    // Use manual SQL construction (workaround for zlay-db parameter binding issue)
    var sql_buf: [1024]u8 = undefined;
    const sql = std.fmt.bufPrint(&sql_buf,
        \\INSERT INTO sessions (client_id, user_id, token_hash, expires_at)
        \\VALUES ('{s}', '{s}', '{s}', to_timestamp({}))
        \\RETURNING id::text
    , .{ client_id, user_id, token_hash, expires_at }) catch return AuthError.DatabaseError;

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
    // Use manual SQL construction (workaround for zlay-db parameter binding issue)
    var sql_buf: [1024]u8 = undefined;
    const sql = std.fmt.bufPrint(&sql_buf,
        \\INSERT INTO projects (user_id, name, description)
        \\VALUES ('{s}', '{s}', '{s}')
        \\RETURNING id::text
    , .{ user_id, name, description }) catch return AuthError.DatabaseError;

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
