<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Button } from '@/components/ui/button'
import { User } from 'lucide-vue-next'

const router = useRouter()

const goToHome = () => {
  router.push('/')
}
const { user, logout } = useAuth()

const handleLogout = async () => {
  await logout()
  router.push('/login')
}

const goToProfile = () => {
  router.push('/profile')
}
</script>

<template>
  <nav class="bg-white shadow-sm border-b border-gray-200">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
      <div class="flex justify-between h-16">
        <!-- Left side - App name -->
        <div class="flex items-center">
          <button
            @click="goToHome"
            class="text-xl font-bold text-gray-900 hover:text-gray-700 transition-colors"
          >
            Zlay
          </button>
        </div>

        <!-- Right side - User menu -->
        <div class="flex items-center">
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" class="flex items-center space-x-2">
                <User class="w-5 h-5" />
                <span>{{ user?.username }}</span>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="w-48">
              <DropdownMenuItem @click="goToProfile">
                Profile
              </DropdownMenuItem>
              <DropdownMenuItem @click="handleLogout" class="text-red-600">
                Logout
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </div>
  </nav>
</template>