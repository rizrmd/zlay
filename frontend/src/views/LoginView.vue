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
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { useRouter, useRoute } from 'vue-router'
import { useAuth } from '@/composables/useAuth'

const router = useRouter()
const route = useRoute()
const { login, isLoading } = useAuth()

const username = ref('')
const password = ref('')
const error = ref('')
const showErrorDialog = ref(false)

const validateUsername = (username: string): boolean => {
  return username.length >= 3
}

const handleSubmit = (event: Event) => {
  console.log('handleSubmit called')
  event.preventDefault()
  event.stopPropagation()
  handleLogin()
}

const handleLogin = async () => {
  console.log('handleLogin called')
  if (!username.value || !password.value) {
    error.value = 'Please fill in all fields'
    showErrorDialog.value = true
    return
  }

  if (!validateUsername(username.value)) {
    error.value = 'Username must be at least 3 characters long'
    showErrorDialog.value = true
    return
  }

  error.value = ''

  const result = await login(username.value, password.value)

  if (result.success) {
    console.log('Login successful, checking for redirect')
    // Check if there's a redirect parameter
    const redirectPath = route.query.redirect as string
    if (redirectPath) {
      console.log('Redirecting to:', redirectPath)
      await router.push(redirectPath)
    } else {
      console.log('No redirect, navigating to dashboard')
      await router.push('/dashboard')
    }
  } else {
    error.value = result.message || 'Login failed'
    showErrorDialog.value = true
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
          Enter your username and password to login
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form @submit="handleSubmit" class="space-y-4">
          <div class="space-y-2">
            <Label for="username">Username</Label>
            <Input id="username" v-model="username" type="text" placeholder="Enter your username" required />
          </div>
          <div class="space-y-2">
            <Label for="password">Password</Label>
            <Input id="password" v-model="password" type="password" placeholder="Enter your password" required />
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

    <AlertDialog :open="showErrorDialog" @update:open="showErrorDialog = $event">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Login Failed</AlertDialogTitle>
          <AlertDialogDescription>
            {{ error }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogAction @click="showErrorDialog = false"> OK </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
