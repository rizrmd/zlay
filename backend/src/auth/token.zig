const std = @import("std");
const crypto = std.crypto;

pub const TokenError = error{
    OutOfMemory,
};

const TOKEN_LEN = 32;
const HASH_HEX_LEN = 64;

/// Generate secure random token
pub fn generateToken(allocator: std.mem.Allocator) ![]const u8 {
    const token = try allocator.alloc(u8, TOKEN_LEN);
    crypto.random.bytes(token);
    return token;
}

/// Hash token for database storage (hex encoded)
pub fn hashToken(allocator: std.mem.Allocator, token: []const u8) ![]const u8 {
    var hasher = crypto.hash.sha2.Sha256.init(.{});
    hasher.update(token);
    var hash: [32]u8 = undefined;
    hasher.final(&hash);

    // Convert to hex string for storage
    const hex_str = try allocator.alloc(u8, HASH_HEX_LEN);
    var i: usize = 0;
    for (hash) |byte| {
        _ = std.fmt.bufPrint(hex_str[i .. i + 2], "{x:0>2}", .{byte}) catch return TokenError.OutOfMemory;
        i += 2;
    }
    return hex_str;
}
