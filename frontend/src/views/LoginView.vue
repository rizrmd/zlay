<script setup lang="ts">
import { ref } from 'vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { useRouter } from 'vue-router'
import { apiClient, type LoginRequest } from '@/services/api'

const router = useRouter()

const email = ref('')
const password = ref('')
const isLoading = ref(false)
const error = ref('')

const validateEmail = (email: string): boolean => {
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
  return emailRegex.test(email)
}

const handleLogin = async () => {
  if (!email.value || !password.value) {
    error.value = 'Please fill in all fields'
    return
  }

  if (!validateEmail(email.value)) {
    error.value = 'Please enter a valid email address'
    return
  }

  isLoading.value = true
  error.value = ''

  try {
    const loginData: LoginRequest = {
      email: email.value,
      password: password.value,
    }

    const response = await apiClient.login(loginData)

    if (response.success) {
      router.push('/dashboard')
    } else {
      error.value = response.message || 'Login failed'
    }
  } catch (err) {
    error.value = 'Network error. Please try again.'
  } finally {
    isLoading.value = false
  }
}

const goToRegister = () => {
  router.push('/register')
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
    <Card class="w-full max-w-md">
      <CardHeader class="space-y-1">
        <CardTitle class="text-2xl text-center"> Sign in to your account </CardTitle>
        <CardDescription class="text-center">
          Enter your email and password to login
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form @submit.prevent="handleLogin" class="space-y-4">
          <div class="space-y-2">
            <Label for="email">Email</Label>
            <Input
              id="email"
              v-model="email"
              type="email"
              placeholder="Enter your email"
              required
            />
          </div>
          <div class="space-y-2">
            <Label for="password">Password</Label>
            <Input
              id="password"
              v-model="password"
              type="password"
              placeholder="Enter your password"
              required
            />
          </div>
          <div v-if="error" class="text-sm text-destructive">
            {{ error }}
          </div>
          <Button type="submit" class="w-full" :disabled="isLoading">
            {{ isLoading ? 'Signing in...' : 'Sign In' }}
          </Button>
        </form>
      </CardContent>
      <CardFooter>
        <p class="text-center text-sm text-gray-600 w-full">
          Don't have an account?
          <Button variant="link" @click="goToRegister" class="p-0 h-auto font-normal">
            Sign up
          </Button>
        </p>
      </CardFooter>
    </Card>
  </div>
</template>
