const std = @import("std");
const pg = @import("pg");
const types = @import("types.zig");

const User = types.User;
const Session = types.Session;
const Project = types.Project;
const Datasource = types.Datasource;
const AuthError = types.AuthError;

/// Register a new user for a specific client
pub fn registerUser(
    allocator: std.mem.Allocator,
    pool: *pg.Pool,
    client_id: []const u8,
    username: []const u8,
    password_hash: []const u8,
) !User {
    std.log.info("database.registerUser: client_id={s}, username={s}", .{ client_id, username });

    // Check if user already exists for this client
    std.log.info("Checking if user already exists", .{});
    const check_result = pool.query(
        \\SELECT 1 FROM users WHERE client_id = $1 AND username = $2 LIMIT 1
    , .{ client_id, username }) catch |err| {
        std.log.err("User existence check failed: {}", .{err});
        return AuthError.DatabaseError;
    };
    defer check_result.deinit();

    if (try check_result.next()) |_| {
        std.log.info("User already exists", .{});
        return AuthError.UserAlreadyExists;
    }

    // Create user
    std.log.info("Creating new user", .{});
    _ = pool.exec(
        \\INSERT INTO users (client_id, username, password_hash)
        \\VALUES ($1, $2, $3)
    , .{ client_id, username, password_hash }) catch |err| {
        std.log.err("User insertion failed: {}", .{err});
        return AuthError.DatabaseError;
    };

    // Fetch created user
    std.log.info("Fetching created user", .{});
    const select_result = pool.query(
        \\SELECT id::text, client_id::text, username, password_hash, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at, is_active
        \\FROM users WHERE client_id = $1 AND username = $2
    , .{ client_id, username }) catch |err| {
        std.log.err("User fetch failed: {}", .{err});
        return AuthError.DatabaseError;
    };
    defer select_result.deinit();

    const row = (try select_result.next()) orelse {
        std.log.err("No rows returned after user creation", .{});
        return AuthError.DatabaseError;
    };

    std.log.info("User created successfully", .{});
    return User{
        .id = allocator.dupe(u8, row.get([]const u8, 0)) catch return AuthError.DatabaseError,
        .client_id = allocator.dupe(u8, row.get([]const u8, 1)) catch return AuthError.DatabaseError,
        .username = allocator.dupe(u8, row.get([]const u8, 2)) catch return AuthError.DatabaseError,
        .password_hash = allocator.dupe(u8, row.get([]const u8, 3)) catch return AuthError.DatabaseError,
        .created_at = allocator.dupe(u8, row.get([]const u8, 4)) catch return AuthError.DatabaseError,
        .is_active = row.get(bool, 5),
    };
}

/// Get user by client_id and username
pub fn getUserByCredentials(
    allocator: std.mem.Allocator,
    pool: *pg.Pool,
    client_id: []const u8,
    username: []const u8,
) !User {
    const user_result = pool.query(
        \\SELECT id::text, client_id::text, username, password_hash, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at, is_active
        \\FROM users WHERE client_id = $1 AND username = $2 AND is_active = true
    , .{ client_id, username }) catch return AuthError.DatabaseError;
    defer user_result.deinit();

    const row = (try user_result.next()) orelse return AuthError.UserNotFound;

    return User{
        .id = allocator.dupe(u8, row.get([]const u8, 0)) catch return AuthError.DatabaseError,
        .client_id = allocator.dupe(u8, row.get([]const u8, 1)) catch return AuthError.DatabaseError,
        .username = allocator.dupe(u8, row.get([]const u8, 2)) catch return AuthError.DatabaseError,
        .password_hash = allocator.dupe(u8, row.get([]const u8, 3)) catch return AuthError.DatabaseError,
        .created_at = allocator.dupe(u8, row.get([]const u8, 4)) catch return AuthError.DatabaseError,
        .is_active = row.get(bool, 5),
    };
}

/// Create session in database
pub fn createSession(
    allocator: std.mem.Allocator,
    pool: *pg.Pool,
    client_id: []const u8,
    user_id: []const u8,
    token_hash: []const u8,
    expires_at: i64,
) ![]const u8 {
    std.log.info("Creating session: client_id={s}, user_id={s}, expires_at={}", .{ client_id, user_id, expires_at });
    std.log.info("Token hash length: {}", .{token_hash.len});

    // Convert expires_at to string for binding
    const expires_at_str = try std.fmt.allocPrint(allocator, "{}", .{expires_at});
    defer allocator.free(expires_at_str);

    // Insert session into database
    std.log.info("About to execute session INSERT with values: client_id='{s}', user_id='{s}', token_hash_len={}, expires_at={s}", .{ client_id, user_id, token_hash.len, expires_at_str });

    const exec_result = pool.exec(
        \\INSERT INTO sessions (client_id, user_id, token_hash, expires_at)
        \\VALUES ($1, $2, $3, to_timestamp($4))
    , .{ client_id, user_id, token_hash, expires_at_str }) catch |err| {
        std.log.err("Session exec failed with error: {s} - client_id='{s}', user_id='{s}', token_hash_len={}, expires_at={s}", .{ @errorName(err), client_id, user_id, token_hash.len, expires_at_str });
        return AuthError.DatabaseError;
    };

    std.log.info("Session exec succeeded, rows affected: {any}", .{exec_result});

    // Query to get the session ID that was just created
    const session_result = pool.query(
        \\SELECT id::text FROM sessions
        \\WHERE client_id = $1 AND user_id = $2 AND token_hash = $3
        \\ORDER BY created_at DESC LIMIT 1
    , .{ client_id, user_id, token_hash }) catch |err| {
        std.log.err("Session query failed: {}", .{err});
        return AuthError.DatabaseError;
    };
    defer session_result.deinit();

    const row = (try session_result.next()) orelse return AuthError.DatabaseError;
    return allocator.dupe(u8, row.get([]const u8, 0));
}

/// Validate session token and return user
pub fn validateSession(
    allocator: std.mem.Allocator,
    pool: *pg.Pool,
    token_hash: []const u8,
) !User {
    const current_time = std.time.timestamp();

    const current_time_str = try std.fmt.allocPrint(allocator, "{}", .{current_time});
    defer allocator.free(current_time_str);

    const session_result = pool.query(
        \\SELECT u.id::text, u.client_id::text, u.username, u.password_hash,
        \\       to_char(u.created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at,
        \\       u.is_active
        \\FROM users u
        \\JOIN sessions s ON u.id::text = s.user_id
        \\WHERE s.token_hash = $1
        \\  AND s.expires_at > to_timestamp($2)
        \\  AND u.is_active = true
    , .{ token_hash, current_time_str }) catch return AuthError.DatabaseError;
    defer session_result.deinit();

    const row = (try session_result.next()) orelse return AuthError.InvalidToken;

    return User{
        .id = allocator.dupe(u8, row.get([]const u8, 0)) catch return AuthError.DatabaseError,
        .client_id = allocator.dupe(u8, row.get([]const u8, 1)) catch return AuthError.DatabaseError,
        .username = allocator.dupe(u8, row.get([]const u8, 2)) catch return AuthError.DatabaseError,
        .password_hash = allocator.dupe(u8, row.get([]const u8, 3)) catch return AuthError.DatabaseError,
        .created_at = allocator.dupe(u8, row.get([]const u8, 4)) catch return AuthError.DatabaseError,
        .is_active = row.get(bool, 5),
    };
}

/// Logout user by invalidating session
pub fn deleteSession(
    pool: *pg.Pool,
    token_hash: []const u8,
) !void {
    _ = pool.exec(
        \\DELETE FROM sessions WHERE token_hash = $1
    , .{token_hash}) catch return AuthError.DatabaseError;
}

/// Clean up expired sessions
pub fn cleanupExpiredSessions(pool: *pg.Pool) !void {
    const current_time = std.time.timestamp();
    _ = pool.exec(
        \\DELETE FROM sessions WHERE expires_at <= to_timestamp($1)
    , .{current_time}) catch return AuthError.DatabaseError;
}

/// Create a new project
pub fn createProject(
    allocator: std.mem.Allocator,
    pool: *pg.Pool,
    user_id: []const u8,
    name: []const u8,
    description: []const u8,
) ![]const u8 {
    std.log.info("database.createProject: user_id={s}, name={s}", .{ user_id, name });

    // Insert project
    _ = pool.exec(
        \\INSERT INTO projects (user_id, name, description)
        \\VALUES ($1, $2, $3)
    , .{ user_id, name, description }) catch |err| {
        std.log.err("Project insertion failed: {}", .{err});
        return AuthError.DatabaseError;
    };

    // Get the created project ID
    const result = pool.query(
        \\SELECT id::text FROM projects
        \\WHERE user_id = $1 AND name = $2
        \\ORDER BY created_at DESC LIMIT 1
    , .{ user_id, name }) catch |err| {
        std.log.err("Project fetch failed: {}", .{err});
        return AuthError.DatabaseError;
    };
    defer result.deinit();

    const row = (try result.next()) orelse return AuthError.DatabaseError;
    return allocator.dupe(u8, row.get([]const u8, 0));
}

/// Get projects by user ID
pub fn getProjectsByUser(
    allocator: std.mem.Allocator,
    pool: *pg.Pool,
    user_id: []const u8,
) ![]Project {
    const result = pool.query(
        \\SELECT id::text, user_id::text, name, description, is_active,
        \\       to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at
        \\FROM projects WHERE user_id = $1 AND is_active = true
        \\ORDER BY created_at DESC
    , .{user_id}) catch return AuthError.DatabaseError;
    defer result.deinit();

    var projects = try std.ArrayList(Project).initCapacity(allocator, 0);
    defer projects.deinit(allocator);

    while (try result.next()) |row| {
        try projects.append(allocator, Project{
            .id = try allocator.dupe(u8, row.get([]const u8, 0)),
            .user_id = try allocator.dupe(u8, row.get([]const u8, 1)),
            .name = try allocator.dupe(u8, row.get([]const u8, 2)),
            .description = try allocator.dupe(u8, row.get([]const u8, 3)),
            .is_active = row.get(bool, 4),
            .created_at = try allocator.dupe(u8, row.get([]const u8, 5)),
        });
    }

    return projects.toOwnedSlice(allocator);
}

/// Get project by ID
pub fn getProjectById(
    allocator: std.mem.Allocator,
    pool: *pg.Pool,
    project_id: []const u8,
) !Project {
    const result = pool.query(
        \\SELECT id::text, user_id::text, name, description, is_active,
        \\       to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at
        \\FROM projects WHERE id = $1 AND is_active = true
    , .{project_id}) catch return AuthError.DatabaseError;
    defer result.deinit();

    const row = (try result.next()) orelse return AuthError.DatabaseError;
    return Project{
        .id = try allocator.dupe(u8, row.get([]const u8, 0)),
        .user_id = try allocator.dupe(u8, row.get([]const u8, 1)),
        .name = try allocator.dupe(u8, row.get([]const u8, 2)),
        .description = try allocator.dupe(u8, row.get([]const u8, 3)),
        .is_active = row.get(bool, 4),
        .created_at = try allocator.dupe(u8, row.get([]const u8, 5)),
    };
}

/// Update project
pub fn updateProject(
    pool: *pg.Pool,
    project_id: []const u8,
    name: []const u8,
    description: []const u8,
) !void {
    _ = pool.exec(
        \\UPDATE projects SET name = $1, description = $2
        \\WHERE id = $3
    , .{ name, description, project_id }) catch return AuthError.DatabaseError;
}

/// Delete project (soft delete)
pub fn deleteProject(pool: *pg.Pool, project_id: []const u8) !void {
    _ = pool.exec(
        \\UPDATE projects SET is_active = false WHERE id = $1
    , .{project_id}) catch return AuthError.DatabaseError;
}

/// Create a new datasource
pub fn createDatasource(
    allocator: std.mem.Allocator,
    pool: *pg.Pool,
    project_id: []const u8,
    name: []const u8,
    ds_type: []const u8,
    config: []const u8,
) ![]const u8 {
    std.log.info("database.createDatasource: project_id={s}, name={s}", .{ project_id, name });

    // Insert datasource
    _ = pool.exec(
        \\INSERT INTO datasources (project_id, name, type, config)
        \\VALUES ($1, $2, $3, $4::jsonb)
    , .{ project_id, name, ds_type, config }) catch |err| {
        std.log.err("Datasource insertion failed: {}", .{err});
        return AuthError.DatabaseError;
    };

    // Get the created datasource ID
    const result = pool.query(
        \\SELECT id::text FROM datasources
        \\WHERE project_id = $1 AND name = $2
        \\ORDER BY created_at DESC LIMIT 1
    , .{ project_id, name }) catch |err| {
        std.log.err("Datasource fetch failed: {}", .{err});
        return AuthError.DatabaseError;
    };
    defer result.deinit();

    const row = (try result.next()) orelse return AuthError.DatabaseError;
    return allocator.dupe(u8, row.get([]const u8, 0));
}

/// Get datasources by project ID
pub fn getDatasourcesByProject(
    allocator: std.mem.Allocator,
    pool: *pg.Pool,
    project_id: []const u8,
) ![]Datasource {
    const result = pool.query(
        \\SELECT id::text, project_id::text, name, type, config::text, is_active,
        \\       to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at
        \\FROM datasources WHERE project_id = $1 AND is_active = true
        \\ORDER BY created_at DESC
    , .{project_id}) catch return AuthError.DatabaseError;
    defer result.deinit();

    var datasources = try std.ArrayList(Datasource).initCapacity(allocator, 0);
    defer datasources.deinit(allocator);

    while (try result.next()) |row| {
        try datasources.append(allocator, Datasource{
            .id = try allocator.dupe(u8, row.get([]const u8, 0)),
            .project_id = try allocator.dupe(u8, row.get([]const u8, 1)),
            .name = try allocator.dupe(u8, row.get([]const u8, 2)),
            .type = try allocator.dupe(u8, row.get([]const u8, 3)),
            .config = try allocator.dupe(u8, row.get([]const u8, 4)),
            .is_active = row.get(bool, 5),
            .created_at = try allocator.dupe(u8, row.get([]const u8, 6)),
        });
    }

    return datasources.toOwnedSlice(allocator);
}

/// Get datasource by ID
pub fn getDatasourceById(
    allocator: std.mem.Allocator,
    pool: *pg.Pool,
    datasource_id: []const u8,
) !Datasource {
    const result = pool.query(
        \\SELECT id::text, project_id::text, name, type, config::text, is_active,
        \\       to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at
        \\FROM datasources WHERE id = $1 AND is_active = true
    , .{datasource_id}) catch return AuthError.DatabaseError;
    defer result.deinit();

    const row = (try result.next()) orelse return AuthError.DatabaseError;
    return Datasource{
        .id = try allocator.dupe(u8, row.get([]const u8, 0)),
        .project_id = try allocator.dupe(u8, row.get([]const u8, 1)),
        .name = try allocator.dupe(u8, row.get([]const u8, 2)),
        .type = try allocator.dupe(u8, row.get([]const u8, 3)),
        .config = try allocator.dupe(u8, row.get([]const u8, 4)),
        .is_active = row.get(bool, 5),
        .created_at = try allocator.dupe(u8, row.get([]const u8, 6)),
    };
}

/// Update datasource
pub fn updateDatasource(
    pool: *pg.Pool,
    datasource_id: []const u8,
    name: []const u8,
    ds_type: []const u8,
    config: []const u8,
) !void {
    _ = pool.exec(
        \\UPDATE datasources SET name = $1, type = $2, config = $3::jsonb
        \\WHERE id = $4
    , .{ name, ds_type, config, datasource_id }) catch return AuthError.DatabaseError;
}

/// Delete datasource (soft delete)
pub fn deleteDatasource(pool: *pg.Pool, datasource_id: []const u8) !void {
    _ = pool.exec(
        \\UPDATE datasources SET is_active = false WHERE id = $1
    , .{datasource_id}) catch return AuthError.DatabaseError;
}
