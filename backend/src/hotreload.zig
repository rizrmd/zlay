const std = @import("std");
const print = std.debug.print;
const fs = std.fs;
const process = std.process;
const time = std.time;

const HotReloadWatcher = struct {
    allocator: std.mem.Allocator,
    server_process: ?process.Child,
    last_modified: i128,

    fn init(allocator: std.mem.Allocator) HotReloadWatcher {
        return HotReloadWatcher{
            .allocator = allocator,
            .server_process = null,
            .last_modified = 0,
        };
    }

    fn getFileModifiedTime(file_path: []const u8) !i128 {
        const file = try fs.cwd().openFile(file_path, .{});
        defer file.close();
        const stat = try file.stat();
        return stat.mtime * std.time.ns_per_s;
    }

    fn startServer(self: *HotReloadWatcher) !void {
        if (self.server_process) |*proc| {
            _ = proc.kill() catch {};
            _ = proc.wait() catch {};
        }

        const argv = [_][]const u8{"./zig-out/bin/webserver"};
        self.server_process = process.Child.init(&argv, self.allocator);

        try self.server_process.?.spawn();
        print("Server started (PID: {})\n", .{self.server_process.?.id});
    }

    fn stopServer(self: *HotReloadWatcher) void {
        if (self.server_process) |*proc| {
            print("Stopping server...\n", .{});
            _ = proc.kill() catch {};
            _ = proc.wait() catch {};
            self.server_process = null;
        }
    }

    fn rebuildServer(self: *HotReloadWatcher) !void {
        print("Rebuilding server...\n", .{});

        const argv = [_][]const u8{ "zig", "build", "-Dtarget=native", "-Doptimize=Debug" };
        var child = process.Child.init(&argv, self.allocator);

        child.stdout_behavior = .Pipe;
        child.stderr_behavior = .Pipe;

        try child.spawn();

        var stdout_buffer: [1024]u8 = undefined;
        var stderr_buffer: [1024]u8 = undefined;

        while (true) {
            const stdout_bytes = try child.stdout.?.read(&stdout_buffer);
            if (stdout_bytes > 0) {
                print("{s}", .{stdout_buffer[0..stdout_bytes]});
            }

            const stderr_bytes = try child.stderr.?.read(&stderr_buffer);
            if (stderr_bytes > 0) {
                print("{s}", .{stderr_buffer[0..stderr_bytes]});
            }

            const term = try child.wait();
            if (term.Exited != 0) {
                print("Build failed with exit code {}\n", .{term.Exited});
                return error.BuildFailed;
            }

            if (stdout_bytes == 0 and stderr_bytes == 0) break;
        }

        print("Build successful!\n", .{});
    }

    fn watch(self: *HotReloadWatcher) !void {
        const source_files = [_][]const u8{ "src/main.zig", "src/hotreload.zig" };
        var last_modified_times = try self.allocator.alloc(i128, source_files.len);
        defer self.allocator.free(last_modified_times);

        for (source_files, 0..) |file, i| {
            last_modified_times[i] = try HotReloadWatcher.getFileModifiedTime(file);
        }

        try self.startServer();

        print("Hot reload enabled. Watching source files for changes...\n", .{});
        print("Press Ctrl+C to stop.\n", .{});

        while (true) {
            // Simple delay loop for 1 second
            const start = std.time.nanoTimestamp();
            while (std.time.nanoTimestamp() - start < 1_000_000_000) {}

            var file_changed = false;
            for (source_files, 0..) |file, i| {
                const current_modified = try HotReloadWatcher.getFileModifiedTime(file);
                if (current_modified > last_modified_times[i]) {
                    print("\nFile {s} changed! Restarting server...\n", .{file});
                    last_modified_times[i] = current_modified;
                    file_changed = true;
                    break;
                }
            }

            if (file_changed) {
                self.stopServer();
                self.rebuildServer() catch |err| {
                    print("Rebuild failed: {}\n", .{err});
                    continue;
                };
                self.startServer() catch |err| {
                    print("Failed to start server: {}\n", .{err});
                };
            }
        }
    }

    fn deinit(self: *HotReloadWatcher) void {
        self.stopServer();
    }
};

pub fn main() !void {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    var watcher = HotReloadWatcher.init(allocator);
    defer watcher.deinit();

    watcher.watch() catch |err| {
        print("Hot reload error: {}\n", .{err});
        return err;
    };
}
