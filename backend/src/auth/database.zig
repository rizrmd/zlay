const std = @import("std");
const pg = @import("pg");
const types = @import("types.zig");

const User = types.User;
const Session = types.Session;
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
        \\SELECT id::text, client_id::text, username, password_hash, created_at, is_active 
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
        \\SELECT id::text, client_id::text, username, password_hash, created_at, is_active
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

    // Try using exec instead of query first to see if it's a query-specific issue
    const exec_result = pool.exec(
        \\INSERT INTO sessions (client_id, user_id, token_hash, expires_at)
        \\VALUES ($1::uuid, $2::uuid, $3, to_timestamp($4))
    , .{ client_id, user_id, token_hash, expires_at }) catch |err| {
        std.log.err("Session exec failed: {}", .{err});
        return AuthError.DatabaseError;
    };

    std.log.info("Session exec succeeded, rows affected: {any}", .{exec_result});

    // Now get the session ID that was created
    const session_result = pool.query(
        \\SELECT id::text FROM sessions 
        \\WHERE client_id = $1::uuid AND user_id = $2::uuid AND token_hash = $3
        \\ORDER BY created_at DESC LIMIT 1
    , .{ client_id, user_id, token_hash }) catch |err| {
        std.log.err("Session query failed: {}", .{err});
        return AuthError.DatabaseError;
    };
    defer session_result.deinit();

    const row = (try session_result.next()) orelse {
        std.log.err("No session found after creation", .{});
        return AuthError.DatabaseError;
    };

    const session_id = allocator.dupe(u8, row.get([]const u8, 0)) catch return AuthError.DatabaseError;
    std.log.info("Session created with ID: {s}", .{session_id});
    return session_id;
}

/// Validate session token and return user
pub fn validateSession(
    allocator: std.mem.Allocator,
    pool: *pg.Pool,
    token_hash: []const u8,
) !User {
    const current_time = std.time.timestamp();

    const session_result = pool.query(
        \\SELECT u.id::text, u.client_id::text, u.username, u.password_hash,
        \\       EXTRACT(EPOCH FROM u.created_at)::text as created_at,
        \\       u.is_active
        \\FROM users u
        \\JOIN sessions s ON u.id = s.user_id
        \\WHERE s.token_hash = $1 
        \\  AND s.expires_at > to_timestamp($2)
        \\  AND u.is_active = true
    , .{ token_hash, current_time }) catch return AuthError.DatabaseError;
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
