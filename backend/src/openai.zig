const std = @import("std");
const print = std.debug.print;
const json = std.json;

const OpenAIClient = struct {
    allocator: std.mem.Allocator,
    api_key: []const u8,
    base_url: []const u8,

    const ChatMessage = struct {
        role: []const u8,
        content: []const u8,
    };

    const ChatRequest = struct {
        model: []const u8,
        messages: []ChatMessage,
        max_tokens: ?u32 = null,
        temperature: ?f32 = null,
    };

    const ChatResponse = struct {
        id: []const u8,
        object: []const u8,
        created: i64,
        model: []const u8,
        choices: []struct {
            index: u32,
            message: ChatMessage,
            finish_reason: ?[]const u8,
        },
        usage: struct {
            prompt_tokens: u32,
            completion_tokens: u32,
            total_tokens: u32,
        },
    };

    fn init(allocator: std.mem.Allocator, api_key: []const u8) OpenAIClient {
        return OpenAIClient{
            .allocator = allocator,
            .api_key = api_key,
            .base_url = "https://api.openai.com/v1",
        };
    }

    fn chatCompletion(self: *OpenAIClient, request: ChatRequest) !ChatResponse {
        var url_buffer = std.ArrayList(u8).init(self.allocator);
        defer url_buffer.deinit();
        try url_buffer.appendSlice(self.base_url);
        try url_buffer.appendSlice("/chat/completions");

        var request_body = std.ArrayList(u8).init(self.allocator);
        defer request_body.deinit();

        try json.stringify(request, .{}, request_body.writer());

        var client = std.http.Client{ .allocator = self.allocator };
        defer client.deinit();

        var headers = std.http.Headers.init(self.allocator);
        defer headers.deinit();

        try headers.append("Authorization", try std.fmt.allocPrint(self.allocator, "Bearer {s}", .{self.api_key}));
        try headers.append("Content-Type", "application/json");

        const uri = try std.Uri.parse(url_buffer.items);

        var req = try client.open(.POST, uri, headers, .{});
        defer req.deinit();

        try req.send();
        try req.writeAll(request_body.items);
        try req.finish();

        var response_body = std.ArrayList(u8).init(self.allocator);
        defer response_body.deinit();

        try req.wait();
        try req.reader().readAllArrayList(&response_body, 1024 * 1024);

        if (req.response.status != .ok) {
            print("OpenAI API error: {} - {s}\n", .{ req.response.status, response_body.items });
            return error.ApiError;
        }

        var parsed = try json.parseFromSlice(ChatResponse, self.allocator, response_body.items, .{});
        defer parsed.deinit();

        return parsed.value;
    }

    fn simpleChat(self: *OpenAIClient, message: []const u8) ![]const u8 {
        const messages = [_]ChatMessage{
            .{ .role = "user", .content = message },
        };

        const request = ChatRequest{
            .model = "gpt-3.5-turbo",
            .messages = &messages,
            .max_tokens = 150,
            .temperature = 0.7,
        };

        const response = try self.chatCompletion(request);

        if (response.choices.len > 0) {
            return self.allocator.dupe(u8, response.choices[0].message.content);
        }

        return error.NoResponse;
    }
};

pub fn main() !void {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    const api_key = std.os.getenv("OPENAI_API_KEY") orelse {
        print("Please set OPENAI_API_KEY environment variable\n", .{});
        return error.MissingApiKey;
    };

    var client = OpenAIClient.init(allocator, api_key);

    const response = try client.simpleChat("Hello! Can you explain what Zig is in one sentence?");
    defer allocator.free(response);

    print("OpenAI Response: {s}\n", .{response});
}
