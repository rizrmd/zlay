const std = @import("std");
const crypto = std.crypto;

pub const TokenError = error{
    OutOfMemory,
};

const TOKEN_LEN = 32;
const HASH_HEX_LEN = 64;

/// Generate secure random token (UUID v4)
pub fn generateToken(allocator: std.mem.Allocator) ![]const u8 {
    var uuid: [16]u8 = undefined;
    crypto.random.bytes(&uuid);

    // Set version bits (4) and variant bits
    uuid[6] = (uuid[6] & 0x0F) | 0x40;
    uuid[8] = (uuid[8] & 0x3F) | 0x80;

    // Format as UUID string
    return std.fmt.allocPrint(allocator, "{x:0>2}{x:0>2}{x:0>2}{x:0>2}-{x:0>2}{x:0>2}-{x:0>2}{x:0>2}-{x:0>2}{x:0>2}-{x:0>2}{x:0>2}{x:0>2}{x:0>2}{x:0>2}{x:0>2}", .{
        uuid[0],  uuid[1],  uuid[2],  uuid[3],
        uuid[4],  uuid[5],  uuid[6],  uuid[7],
        uuid[8],  uuid[9],  uuid[10], uuid[11],
        uuid[12], uuid[13], uuid[14], uuid[15],
    });
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
