<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { useAuth } from '@/composables/useAuth'
import { apiClient } from '@/services/api'
import ProjectCard from '@/components/ProjectCard.vue'
import Navbar from '@/components/Navbar.vue'
import type { Project } from '@/services/api'

const { user, isLoading, logout, checkAuth } = useAuth()

const projects = ref<Project[]>([])
const projectsLoading = ref(false)
const projectsError = ref<string | null>(null)

// Create project form state
const isCreating = ref(false)
const projectName = ref('')
const createLoading = ref(false)

const handleLogout = async () => {
  await logout()
  window.location.href = '/login'
}

const loadProjects = async () => {
  try {
    projectsLoading.value = true
    projectsError.value = null
    const apiProjects = await apiClient.getProjects()
    projects.value = apiProjects || []
  } catch (error) {
    console.error('Failed to load projects:', error)
    projectsError.value = 'Failed to load projects'
  } finally {
    projectsLoading.value = false
  }
}

const handleAddMember = (project: Project) => {
  console.log('Add member to project:', project.name)
  // TODO: Implement add member functionality
}

const handleEditProject = (project: Project) => {
  console.log('Edit project:', project.name)
  // TODO: Implement edit project functionality
}

const router = useRouter()

const navigateToChat = (project: Project) => {
  router.push(`/p/${project.id}/chat`)
}

const handleDeleteProject = (project: Project) => {
  if (confirm(`Are you sure you want to delete "${project.name}"?`)) {
    console.log('Delete project:', project.name)
    // TODO: Implement delete project functionality
  }
}

const handleCreateProject = () => {
  isCreating.value = true
  projectName.value = ''
}

const handleSubmitProject = async () => {
  if (!projectName.value.trim()) {
    return
  }

  createLoading.value = true
  try {
    await apiClient.createProject(projectName.value.trim(), '')
    isCreating.value = false
    projectName.value = ''
    await loadProjects()
  } catch (error) {
    console.error('Failed to create project:', error)
  } finally {
    createLoading.value = false
  }
}

const handleCancelCreate = () => {
  isCreating.value = false
  projectName.value = ''
}

onMounted(async () => {
  // Router guard already handles authentication check, no need to call again
  await loadProjects()
})
</script>

<template>
  <div class="min-h-screen bg-gray-50">
    <Navbar />
    <div class="max-w-7xl mx-auto py-8 px-4 sm:px-6 lg:px-8">

    <div v-if="isLoading" class="text-center">
      <p>Loading...</p>
    </div>

    <!-- User Dashboard -->
    <div v-if="user" class="space-y-6">
      <!-- Projects Section -->
      <div>
        <h2 class="text-2xl font-bold mb-6">Projects</h2>

        <div v-if="projectsLoading" class="text-center py-8">
          <p>Loading projects...</p>
        </div>

        <div v-else-if="projectsError" class="text-center py-8">
          <p class="text-red-600">{{ projectsError }}</p>
          <Button @click="loadProjects" variant="outline" class="mt-2"> Try Again </Button>
        </div>

        <div v-else-if="!projects || projects.length === 0" class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          <!-- Create Project Card (only card when no projects) -->
          <Card
            class="border-dashed border-2 border-gray-300 hover:border-blue-400 transition-colors duration-200 h-full flex flex-col"
          >
            <CardContent class="p-6 flex-1 flex flex-col">
              <!-- Default state -->
              <div
                v-if="!isCreating"
                class="flex flex-col items-center justify-center text-center h-full cursor-pointer"
                @click="handleCreateProject"
              >
                <svg
                  class="w-12 h-12 text-gray-400 mb-3 hover:text-blue-500 transition-colors"
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 4v16m8-8H4"
                  />
                </svg>
                <p class="text-lg font-medium text-gray-600 hover:text-blue-600 transition-colors">
                  Create Project
                </p>
              </div>

              <!-- Form state -->
              <div v-else class="flex flex-col items-center justify-center h-full">
                <div class="w-full max-w-xs space-y-4">
                  <Input
                    v-model="projectName"
                    placeholder="Enter project name"
                    @keyup.enter="handleSubmitProject"
                    ref="projectNameInput"
                  />
                  <div class="flex space-x-2">
                    <Button
                      variant="outline"
                      @click="handleCancelCreate"
                      :disabled="createLoading"
                      class="flex-1"
                    >
                      Cancel
                    </Button>
                    <Button
                      @click="handleSubmitProject"
                      :disabled="createLoading || !projectName.trim()"
                      class="flex-1"
                    >
                      <span v-if="createLoading">Creating...</span>
                      <span v-else>Submit</span>
                    </Button>
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        <div v-else class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          <!-- Project Cards -->
          <ProjectCard
            v-for="project in (projects || [])"
            :key="project.id"
            :project="project"
            :chat-count="0"
            :datasource-count="0"
            @add-member="handleAddMember"
            @edit-project="handleEditProject"
            @delete-project="handleDeleteProject"
            @click="navigateToChat"
          />

          <!-- Create Project Card (always at the end) -->
          <Card
            class="border-dashed border-2 border-gray-300 hover:border-blue-400 transition-colors duration-200 h-full flex flex-col"
          >
            <CardContent class="p-6 flex-1 flex flex-col">
              <!-- Default state -->
              <div
                v-if="!isCreating"
                class="flex flex-col items-center justify-center text-center h-full cursor-pointer"
                @click="handleCreateProject"
              >
                <svg
                  class="w-12 h-12 text-gray-400 mb-3 hover:text-blue-500 transition-colors"
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 4v16m8-8H4"
                  />
                </svg>
                <p class="text-lg font-medium text-gray-600 hover:text-blue-600 transition-colors">
                  Create Project
                </p>
              </div>

              <!-- Form state -->
              <div v-else class="flex flex-col items-center justify-center h-full">
                <div class="w-full max-w-xs space-y-4">
                  <Input
                    v-model="projectName"
                    placeholder="Enter project name"
                    @keyup.enter="handleSubmitProject"
                    ref="projectNameInput"
                  />
                  <div class="flex space-x-2">
                    <Button
                      variant="outline"
                      @click="handleCancelCreate"
                      :disabled="createLoading"
                      class="flex-1"
                    >
                      Cancel
                    </Button>
                    <Button
                      @click="handleSubmitProject"
                      :disabled="createLoading || !projectName.trim()"
                      class="flex-1"
                    >
                      <span v-if="createLoading">Creating...</span>
                      <span v-else>Submit</span>
                    </Button>
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>

      <div v-else class="text-center py-8">
        <p class="text-red-600">Failed to load dashboard</p>
      </div>
    </div>
  </div>
</template>
