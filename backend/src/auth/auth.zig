const std = @import("std");
const pg = @import("pg");

const types = @import("types.zig");
const password = @import("password.zig");
const token = @import("token.zig");
const database = @import("database.zig");

const User = types.User;
const LoginResult = types.LoginResult;
const RegisterRequest = types.RegisterRequest;
const LoginRequest = types.LoginRequest;
const AuthError = types.AuthError;

/// Register a new user
pub fn registerUser(
    allocator: std.mem.Allocator,
    pool: *pg.Pool,
    req: RegisterRequest,
) !User {
    std.log.info("Starting registration for user '{s}' with client '{s}'", .{ req.username, req.client_id });

    const password_hash = try password.hashPassword(allocator, req.password);
    defer allocator.free(password_hash);

    std.log.info("Password hashed successfully, calling database.registerUser", .{});
    return database.registerUser(allocator, pool, req.client_id, req.username, password_hash);
}

/// Authenticate user and create session
pub fn loginUser(
    allocator: std.mem.Allocator,
    pool: *pg.Pool,
    req: LoginRequest,
) !LoginResult {
    // Get user
    const user = try database.getUserByCredentials(allocator, pool, req.client_id, req.username);

    // Verify password
    const is_valid = try password.verifyPassword(req.password, user.password_hash);
    if (!is_valid) {
        return AuthError.InvalidPassword;
    }

    // Generate session token
    const session_token = try token.generateToken(allocator);
    const token_hash = try token.hashToken(allocator, session_token);
    defer allocator.free(token_hash);

    // Create session (expires in 24 hours)
    const session_expires_at = std.time.timestamp() + (24 * 60 * 60);
    const session_id = database.createSession(allocator, pool, user.client_id, user.id, token_hash, session_expires_at) catch |err| {
        std.log.err("Failed to create session: {}", .{err});
        // For now, continue without session storage
        return LoginResult{
            .user = user,
            .token = session_token,
            .session_id = "",
        };
    };

    return LoginResult{
        .user = user,
        .token = session_token,
        .session_id = session_id,
    };
}

/// Validate session token and return user
pub fn validateSession(
    allocator: std.mem.Allocator,
    pool: *pg.Pool,
    session_token: []const u8,
) !User {
    const token_hash = try token.hashToken(allocator, session_token);
    defer allocator.free(token_hash);

    return database.validateSession(allocator, pool, token_hash);
}

/// Logout user by invalidating session
pub fn logoutUser(
    allocator: std.mem.Allocator,
    pool: *pg.Pool,
    session_token: []const u8,
) !void {
    const token_hash = try token.hashToken(allocator, session_token);
    defer allocator.free(token_hash);

    try database.deleteSession(pool, token_hash);
}

/// Clean up expired sessions
pub fn cleanupExpiredSessions(pool: *pg.Pool) !void {
    try database.cleanupExpiredSessions(pool);
}
