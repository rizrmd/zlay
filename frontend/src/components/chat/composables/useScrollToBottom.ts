import { ref, nextTick } from 'vue'

export const useScrollToBottom = () => {
  const messagesContainer = ref<HTMLElement | null>(null)

  const scrollToBottom = () => {
    if (messagesContainer.value) {
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    }
  }

  const scrollToBottomAsync = async () => {
    await nextTick()
    scrollToBottom()
  }

  return {
    messagesContainer,
    scrollToBottom,
    scrollToBottomAsync
  }
}