import { computed, ref } from 'vue'

export const useMobileDetection = () => {
  const isMobile = ref(false)
  const sidebarOpen = ref(true)

  const checkMobile = () => {
    const wasMobile = isMobile.value
    isMobile.value = window.innerWidth < 768

    if (wasMobile && !isMobile.value) {
      sidebarOpen.value = true
    } else if (isMobile.value) {
      sidebarOpen.value = false
    }
  }

  const toggleSidebar = () => {
    sidebarOpen.value = !sidebarOpen.value
  }

  return {
    isMobile,
    sidebarOpen: computed(() => sidebarOpen.value),
    checkMobile,
    toggleSidebar
  }
}