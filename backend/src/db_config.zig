const std = @import("std");
const zlay_db = @import("zlay-db");

pub const DatabaseConfig = struct {
    allocator: std.mem.Allocator,
    database_type: zlay_db.DatabaseType,
    connection_string: []const u8,

    // For PostgreSQL/MySQL
    host: ?[]const u8 = null,
    port: ?u16 = null,
    database: ?[]const u8 = null,
    username: ?[]const u8 = null,
    password: ?[]const u8 = null,

    // For SQLite
    file_path: ?[]const u8 = null,

    pub fn fromEnvironment(allocator: std.mem.Allocator) !DatabaseConfig {
        // Get database URL from environment
        const database_url = try getEnvVar(allocator, "DATABASE_URL");

        // Determine database type from URL
        if (std.mem.startsWith(u8, database_url, "postgresql://") or
            std.mem.startsWith(u8, database_url, "postgres://"))
        {
            return parsePostgresUrl(allocator, database_url);
        } else if (std.mem.startsWith(u8, database_url, "mysql://")) {
            return parseMysqlUrl(allocator, database_url);
        } else if (std.mem.startsWith(u8, database_url, "sqlite://")) {
            return parseSqliteUrl(allocator, database_url);
        } else if (std.mem.endsWith(u8, database_url, ".db") or
            std.mem.endsWith(u8, database_url, ".sqlite") or
            std.mem.endsWith(u8, database_url, ".sqlite3"))
        {
            return DatabaseConfig{
                .allocator = allocator,
                .database_type = .sqlite,
                .connection_string = database_url,
                .file_path = database_url,
            };
        } else {
            // Default to PostgreSQL for backwards compatibility
            return parsePostgresUrl(allocator, database_url);
        }
    }

    fn parsePostgresUrl(allocator: std.mem.Allocator, url: []const u8) !DatabaseConfig {
        // Parse postgresql://[user[:password]@]host[:port]/database
        var config = DatabaseConfig{
            .allocator = allocator,
            .database_type = .postgresql,
            .connection_string = url,
        };

        // Remove protocol prefix
        const url_without_protocol = if (std.mem.startsWith(u8, url, "postgresql://"))
            url["postgresql://".len..]
        else if (std.mem.startsWith(u8, url, "postgres://"))
            url["postgres://".len..]
        else
            url;

        // Split at @ to separate credentials from host
        var host_part: []const u8 = url_without_protocol;
        var credentials_part: []const u8 = "";

        if (std.mem.indexOf(u8, url_without_protocol, "@")) |at_pos| {
            credentials_part = url_without_protocol[0..at_pos];
            host_part = url_without_protocol[at_pos + 1 ..];
        }

        // Parse credentials
        if (credentials_part.len > 0) {
            if (std.mem.indexOf(u8, credentials_part, ":")) |colon_pos| {
                config.username = try allocator.dupe(u8, credentials_part[0..colon_pos]);
                config.password = try allocator.dupe(u8, credentials_part[colon_pos + 1 ..]);
            } else {
                config.username = try allocator.dupe(u8, credentials_part);
            }
        }

        // Parse host and database
        if (std.mem.indexOf(u8, host_part, "/")) |slash_pos| {
            const host_port = host_part[0..slash_pos];
            config.database = try allocator.dupe(u8, host_part[slash_pos + 1 ..]);

            // Parse port if present
            if (std.mem.indexOf(u8, host_port, ":")) |colon_pos| {
                config.host = try allocator.dupe(u8, host_port[0..colon_pos]);
                const port_str = host_port[colon_pos + 1 ..];
                config.port = try std.fmt.parseInt(u16, port_str, 10);
            } else {
                config.host = try allocator.dupe(u8, host_port);
            }
        } else {
            config.host = try allocator.dupe(u8, host_part);
        }

        // Set defaults
        if (config.host == null) config.host = try allocator.dupe(u8, "localhost");
        if (config.port == null) config.port = 5432;

        return config;
    }

    fn parseMysqlUrl(allocator: std.mem.Allocator, url: []const u8) !DatabaseConfig {
        // Similar to PostgreSQL but with MySQL defaults
        var config = DatabaseConfig{
            .allocator = allocator,
            .database_type = .mysql,
            .connection_string = url,
        };

        // Similar parsing logic as PostgreSQL but with MySQL defaults
        // For now, simplified implementation
        config.host = try allocator.dupe(u8, "localhost");
        config.port = 3306;

        return config;
    }

    fn parseSqliteUrl(allocator: std.mem.Allocator, url: []const u8) !DatabaseConfig {
        const file_path = if (std.mem.startsWith(u8, url, "sqlite://"))
            url["sqlite://".len..]
        else
            url;

        return DatabaseConfig{
            .allocator = allocator,
            .database_type = .sqlite,
            .connection_string = url,
            .file_path = try allocator.dupe(u8, file_path),
        };
    }

    fn getEnvVar(allocator: std.mem.Allocator, key: []const u8) ![]const u8 {
        return std.process.getEnvVarOwned(allocator, key) catch |err| switch (err) {
            error.EnvironmentVariableNotFound => {
                std.log.err("Environment variable '{s}' not found", .{key});
                return error.MissingEnvironmentVariable;
            },
            else => return err,
        };
    }

    pub fn toZlayDbConfig(self: DatabaseConfig) zlay_db.ConnectionConfig {
        return switch (self.database_type) {
            .postgresql => .{
                .database_type = .postgresql,
                .connection_string = self.connection_string,
                .host = self.host,
                .port = self.port,
                .database = self.database,
                .username = self.username,
                .password = self.password,
                .allocator = self.allocator,
            },
            .mysql => .{
                .database_type = .mysql,
                .connection_string = self.connection_string,
                .host = self.host,
                .port = self.port,
                .database = self.database,
                .username = self.username,
                .password = self.password,
                .allocator = self.allocator,
            },
            .sqlite => .{
                .database_type = .sqlite,
                .file_path = self.file_path,
                .allocator = self.allocator,
            },
            else => unreachable,
        };
    }

    pub fn deinit(self: DatabaseConfig) void {
        if (self.host) |host| self.allocator.free(host);
        if (self.database) |database| self.allocator.free(database);
        if (self.username) |username| self.allocator.free(username);
        if (self.password) |password| self.allocator.free(password);
        if (self.file_path) |file_path| self.allocator.free(file_path);
    }
};
