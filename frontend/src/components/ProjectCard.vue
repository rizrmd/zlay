<template>
  <Card class="group hover:shadow-md transition-shadow duration-200 h-full flex flex-col cursor-pointer" @click="handleCardClick">
    <CardHeader class="pb-3">
      <div class="flex items-start justify-between">
        <div class="flex-1 min-w-0">
          <!-- Blue chat icon above title -->
          <div class="flex items-center space-x-2 mb-2">
            <svg class="w-5 h-5 text-blue-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
            </svg>
            <span class="text-sm text-gray-600">Created at {{ formatDate(project.created_at) }}</span>
          </div>
          <CardTitle class="text-lg font-semibold truncate">{{ project.name }}</CardTitle>
        </div>
        
        <div class="flex items-center space-x-2 ml-2 opacity-0 group-hover:opacity-100 transition-opacity duration-200">
          <button 
            @click.stop="navigateToChat"
            class="p-1 text-blue-600 hover:bg-blue-50 rounded transition-colors"
            title="Open Chat"
          >
            <svg class="w-4 h-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
            </svg>
          </button>
          <Dropdown @click.stop>
            <DropdownItem @click="handleAddMember">
                Add Member
              </DropdownItem>
            <DropdownItem @click="handleEditProject">
                Edit Project
              </DropdownItem>
            <DropdownItem @click="handleDeleteProject" class="text-red-600 hover:text-red-700 hover:bg-red-50">
                Delete Project
              </DropdownItem>
          </Dropdown>
        </div>
      </div>
    </CardHeader>
    
    <CardContent class="pt-0 flex-1">
      <!-- Main content area - can be used for future content -->
    </CardContent>
    
    <CardFooter class="pt-0 mt-auto">
      <div class="flex items-center justify-between text-sm text-gray-500 w-full">
        <div class="flex items-center space-x-4">
          
          <div class="flex items-center space-x-1">
            <svg class="w-4 h-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
            </svg>
            <span>{{ chatCount }} chats</span>
          </div>
          
          <div class="flex items-center space-x-1">
            <svg class="w-4 h-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4" />
            </svg>
            <span>{{ datasourceCount }} datasources</span>
          </div>
        </div>
      </div>
    </CardFooter>
  </Card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Dropdown, DropdownItem } from '@/components/ui/dropdown'
import type { Project } from '@/services/api'

interface Props {
  project: Project
  chatCount?: number
  datasourceCount?: number
}

const props = withDefaults(defineProps<Props>(), {
  chatCount: 0,
  datasourceCount: 0
})

const emit = defineEmits<{
  addMember: [project: Project]
  editProject: [project: Project]
  deleteProject: [project: Project]
  click: [project: Project]
}>()

const formatDate = (timestamp: string) => {
  return new Date(parseFloat(timestamp) * 1000).toLocaleDateString()
}

const handleAddMember = () => {
  emit('addMember', props.project)
}

const handleEditProject = () => {
  emit('editProject', props.project)
}

const handleDeleteProject = () => {
  emit('deleteProject', props.project)
}

const handleCardClick = () => {
  emit('click', props.project)
}

const router = useRouter()

const navigateToChat = () => {
  router.push(`/p/${props.project.id}/chat`)
}
</script>