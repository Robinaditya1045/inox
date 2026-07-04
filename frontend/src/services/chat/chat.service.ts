import { apiClient } from '../../api/client';
import type { ChatMessage } from '../../types/chat';

export interface GetMessagesResponse {
  room_id: string;
  messages: ChatMessage[];
  count: number;
}

class ChatService {
  async getRecentMessages(roomId: string, limit = 50): Promise<ChatMessage[]> {
    const res = await apiClient.get<GetMessagesResponse>(`/rooms/${roomId}/messages?limit=${limit}`);
    return res.messages || [];
  }
}

export const chatService = new ChatService();
