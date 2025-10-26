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
import { apiClient, type RegisterRequest } from '@/services/api'

const router = useRouter()

const firstName = ref('')
const lastName = ref('')
const email = ref('')
const password = ref('')
const confirmPassword = ref('')
const isLoading = ref(false)
const error = ref('')

const validateEmail = (email: string): boolean => {
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
  return emailRegex.test(email)
}

const validatePassword = (password: string): boolean => {
  return password.length >= 8
}

const handleRegister = async () => {
  if (!email.value || !password.value) {
    error.value = 'Please fill in all required fields'
    return
  }

  if (!validateEmail(email.value)) {
    error.value = 'Please enter a valid email address'
    return
  }

  if (!validatePassword(password.value)) {
    error.value = 'Password must be at least 8 characters long'
    return
  }

  if (password.value !== confirmPassword.value) {
    error.value = 'Passwords do not match'
    return
  }

  isLoading.value = true
  error.value = ''

  try {
    const registerData: RegisterRequest = {
      email: email.value,
      password: password.value,
      first_name: firstName.value || undefined,
      last_name: lastName.value || undefined,
    }

    const response = await apiClient.register(registerData)

    if (response.success) {
      router.push('/login')
    } else {
      error.value = response.message || 'Registration failed'
    }
  } catch (err) {
    error.value = 'Network error. Please try again.'
  } finally {
    isLoading.value = false
  }
}

const goToLogin = () => {
  router.push('/login')
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
    <Card class="w-full max-w-md">
      <CardHeader class="space-y-1">
        <CardTitle class="text-2xl text-center"> Create your account </CardTitle>
        <CardDescription class="text-center">
          Enter your information to get started
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form @submit.prevent="handleRegister" class="space-y-4">
          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-2">
              <Label for="firstName">First Name</Label>
              <Input id="firstName" v-model="firstName" type="text" placeholder="First name" />
            </div>
            <div class="space-y-2">
              <Label for="lastName">Last Name</Label>
              <Input id="lastName" v-model="lastName" type="text" placeholder="Last name" />
            </div>
          </div>
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
              placeholder="Create a password (min 8 characters)"
              required
            />
          </div>
          <div class="space-y-2">
            <Label for="confirmPassword">Confirm Password</Label>
            <Input
              id="confirmPassword"
              v-model="confirmPassword"
              type="password"
              placeholder="Confirm your password"
              required
            />
          </div>
          <div v-if="error" class="text-sm text-destructive">
            {{ error }}
          </div>
          <Button type="submit" class="w-full" :disabled="isLoading">
            {{ isLoading ? 'Creating account...' : 'Sign Up' }}
          </Button>
        </form>
      </CardContent>
      <CardFooter>
        <p class="text-center text-sm text-gray-600 w-full">
          Already have an account?
          <Button variant="link" @click="goToLogin" class="p-0 h-auto font-normal">
            Sign in
          </Button>
        </p>
      </CardFooter>
    </Card>
  </div>
</template>
