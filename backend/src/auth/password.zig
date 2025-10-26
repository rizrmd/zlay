const std = @import("std");
const crypto = std.crypto;
const base64 = std.base64;

pub const PasswordError = error{
    InvalidHash,
    OutOfMemory,
};

const SALT_LEN = 16;
const HASH_LEN = 32;

/// Hash password using SHA-256 with salt
pub fn hashPassword(allocator: std.mem.Allocator, password: []const u8) ![]const u8 {
    var salt: [SALT_LEN]u8 = undefined;
    crypto.random.bytes(&salt);

    var combined = try allocator.alloc(u8, SALT_LEN + HASH_LEN);
    @memcpy(combined[0..SALT_LEN], &salt);

    // Hash password + salt
    var hasher = crypto.hash.sha2.Sha256.init(.{});
    hasher.update(password);
    hasher.update(&salt);
    var hash: [HASH_LEN]u8 = undefined;
    hasher.final(&hash);
    @memcpy(combined[SALT_LEN..], &hash);

    // Encode as base64 for storage
    const encoded_len = base64.standard.Encoder.calcSize(SALT_LEN + HASH_LEN);
    const encoded = try allocator.alloc(u8, encoded_len);
    _ = base64.standard.Encoder.encode(encoded, combined);
    allocator.free(combined);

    return encoded;
}

/// Verify password against stored hash
pub fn verifyPassword(password: []const u8, stored_hash: []const u8) !bool {
    // Decode base64 stored hash
    const decoded_len = base64.standard.Decoder.calcSizeForSlice(stored_hash) catch return false;
    const decoded = try std.heap.page_allocator.alloc(u8, decoded_len);
    defer std.heap.page_allocator.free(decoded);

    _ = base64.standard.Decoder.decode(decoded, stored_hash) catch return false;

    if (decoded_len != SALT_LEN + HASH_LEN) return false;

    // Extract salt and hash
    const salt = decoded[0..SALT_LEN];
    const stored_password_hash = decoded[SALT_LEN..];

    // Hash provided password with the same salt
    var hasher = crypto.hash.sha2.Sha256.init(.{});
    hasher.update(password);
    hasher.update(salt);
    var computed_hash: [HASH_LEN]u8 = undefined;
    hasher.final(&computed_hash);

    // Compare hashes
    return std.mem.eql(u8, stored_password_hash, &computed_hash);
}
