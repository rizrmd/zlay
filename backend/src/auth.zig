// Main authentication module - clean interface
pub const types = @import("auth/types.zig");
pub const password = @import("auth/password.zig");
pub const token = @import("auth/token.zig");
pub const database = @import("auth/database.zig");
pub const auth = @import("auth/auth.zig");

// Re-export main types and functions for convenience
pub const AuthError = types.AuthError;
pub const User = types.User;
pub const Session = types.Session;
pub const LoginResult = types.LoginResult;
pub const RegisterRequest = types.RegisterRequest;
pub const LoginRequest = types.LoginRequest;

pub const registerUser = auth.registerUser;
pub const loginUser = auth.loginUser;
pub const validateSession = auth.validateSession;
pub const logoutUser = auth.logoutUser;
pub const cleanupExpiredSessions = auth.cleanupExpiredSessions;
pub const hashPassword = password.hashPassword;
pub const verifyPassword = password.verifyPassword;
pub const generateToken = token.generateToken;
pub const hashToken = token.hashToken;
