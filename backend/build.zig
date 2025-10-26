const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    const exe = b.addExecutable(.{
        .name = "zlay-backend",
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/main.zig"),
            .target = target,
            .optimize = optimize,
        }),
    });

    const pg = b.dependency("pg", .{
        .target = target,
        .optimize = optimize,
    });
    exe.root_module.addImport("pg", pg.module("pg"));

    b.installArtifact(exe);

    const run_cmd = b.addRunArtifact(exe);
    run_cmd.step.dependOn(b.getInstallStep());
    if (b.args) |args| {
        run_cmd.addArgs(args);
    }

    const run_step = b.step("run", "Run the app");
    run_step.dependOn(&run_cmd.step);

    // Add hot reload executable for development
    const hotreload_exe = b.addExecutable(.{
        .name = "hotreload",
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/hotreload.zig"),
            .target = target,
            .optimize = .Debug,
        }),
    });

    b.installArtifact(hotreload_exe);

    // Add dev server executable for hot reload
    const dev_server_exe = b.addExecutable(.{
        .name = "webserver",
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/main.zig"),
            .target = target,
            .optimize = .Debug,
        }),
    });

    const dev_server_pg = b.dependency("pg", .{
        .target = target,
        .optimize = .Debug,
    });
    dev_server_exe.root_module.addImport("pg", dev_server_pg.module("pg"));

    b.installArtifact(dev_server_exe);

    // Create dev step that runs hot reload
    const hotreload_run_cmd = b.addRunArtifact(hotreload_exe);
    hotreload_run_cmd.step.dependOn(b.getInstallStep());

    const dev_step = b.step("dev", "Run in development mode with hot reload");
    dev_step.dependOn(&hotreload_run_cmd.step);

    // Add prod command for production with release mode
    const prod_exe = b.addExecutable(.{
        .name = "zlay-backend-prod",
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/main.zig"),
            .target = target,
            .optimize = .ReleaseFast,
        }),
    });

    const prod_pg = b.dependency("pg", .{
        .target = target,
        .optimize = .ReleaseFast,
    });
    prod_exe.root_module.addImport("pg", prod_pg.module("pg"));

    b.installArtifact(prod_exe);

    const prod_step = b.step("prod", "Build for production mode (release build)");
    prod_step.dependOn(b.getInstallStep());
}
