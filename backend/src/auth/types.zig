const std = @import("std");

pub const AuthError = error{
    InvalidPassword,
    UserNotFound,
    UserAlreadyExists,
    InvalidToken,
    SessionExpired,
    DatabaseError,
    InvalidClient,
};

pub const User = struct {
    id: []const u8,
    client_id: []const u8,
    username: []const u8,
    password_hash: []const u8,
    created_at: []const u8,
    is_active: bool,
};

pub const Session = struct {
    id: []const u8,
    client_id: []const u8,
    user_id: []const u8,
    token_hash: []const u8,
    expires_at: []const u8,
    created_at: []const u8,
    last_accessed_at: []const u8,
};

pub const LoginResult = struct {
    user: User,
    token: []const u8,
    session_id: []const u8,
};

pub const RegisterRequest = struct {
    client_id: []const u8,
    username: []const u8,
    password: []const u8,
};

pub const LoginRequest = struct {
    client_id: []const u8,
    username: []const u8,
    password: []const u8,
};

pub const Project = struct {
    id: []const u8,
    user_id: []const u8,
    name: []const u8,
    description: []const u8,
    is_active: bool,
    created_at: []const u8,
};

pub const Datasource = struct {
    id: []const u8,
    project_id: []const u8,
    name: []const u8,
    type: []const u8,
    config: []const u8, // JSON string
    is_active: bool,
    created_at: []const u8,
};
