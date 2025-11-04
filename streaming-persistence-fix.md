# Streaming Persistence Fix Plan

## Issues Identified

1. **Frontend streaming state not persisted**
2. **Backend streaming state exists but not restored on reconnection**
3. **Message duplication when merging history + streaming**
4. **Missing automatic reconnection to active streams**

## Implementation Plan

### Backend Changes (Already Present ✅)
- [x] `activeStreams` map for tracking ongoing conversations
- [x] `GetAllActiveStreams()` method
- [x] `LoadStreamingConversation()` method
- [x] `GetConversationStatus()` method

### Frontend Changes Needed

#### 1. Persist Streaming State in Frontend
- Save active conversation IDs to localStorage
- Save processing state information
- Restore on app initialization

#### 2. Automatic Reconnection Logic
- On WebSocket connect, request all active streams
- Check localStorage for pending conversations
- Automatically rejoin streaming conversations

#### 3. Better Message Merging
- Load complete history from API first
- Then intelligently merge streaming state
- Avoid duplicates by matching message IDs

#### 4. Connection State Management
- Track connection interruptions
- Queue messages during disconnection
- Resume streaming on reconnect

## Key Components to Fix

### chat.ts Store
1. **persistStreamingState()** - Save to localStorage
2. **restoreStreamingState()** - Load from localStorage  
3. **handleReconnection()** - Request active streams on connect
4. **mergeStreamingWithHistory()** - Intelligent message merging

### websocket.ts Service
1. **onConnect hook** - Trigger reconnection logic
2. **requestAllConversationStatuses()** - Already implemented
3. **handleConnectionLoss()** - Queue messages during disconnect

## Implementation Steps

1. Add localStorage persistence to chat store
2. Modify WebSocket init to request active streams
3. Improve message merging in streaming_conversation_loaded handler
4. Add reconnection detection logic
5. Test page refresh during active streaming
6. Test conversation switching during active streaming

## Success Criteria

✅ User starts chat, AI begins streaming
✅ User refreshes page - streaming continues from where it left off
✅ User switches conversations and comes back - sees current streaming state
✅ No duplicate messages appear
✅ Processing indicator shows correctly during reconnection