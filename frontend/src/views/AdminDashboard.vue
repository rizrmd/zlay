<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Eye, Settings, MoreVertical } from 'lucide-vue-next'
import { useAuth } from '@/composables/useAuth'
import { apiClient } from '@/services/api'

interface Client {
  id: string
  name: string
  slug: string
  ai_api_key: string | null
  ai_api_url: string | null
  ai_api_model: string | null
  is_active: boolean
  created_at: string
}

interface Domain {
  id: string
  client_id: string
  domain: string
  is_active: boolean
  created_at: string
}

const { user, logout } = useAuth()

// Clients state
const clients = ref<Client[]>([])
const clientsLoading = ref(false)
const clientsError = ref<string | null>(null)

// Domains state
const domains = ref<Domain[]>([])
const domainsLoading = ref(false)
const domainsError = ref<string | null>(null)

// Create client dialog
const showCreateClient = ref(false)
const newClientName = ref('')
const newClientAiApiKey = ref('')
const newClientAiApiUrl = ref('')
const newClientAiApiModel = ref('')
const createClientLoading = ref(false)

// Create domain dialog
const showCreateDomain = ref(false)
const newDomain = ref('')
const selectedClientForDomain = ref<Client | null>(null)
const createDomainLoading = ref(false)

// View domains modal
const showViewModal = ref(false)
const selectedClient = ref<Client | null>(null)
const clientDomains = ref<Domain[]>([])

// Add domain in modal
const newDomainInModal = ref('')
const addDomainInModalLoading = ref(false)

// Edit client modal
const showEditModal = ref(false)
const editingClient = ref<Client | null>(null)
const editClientName = ref('')
const editClientSlug = ref('')
const editClientAiApiKey = ref('')
const editClientAiApiUrl = ref('')
const editClientAiApiModel = ref('')
const editClientLoading = ref(false)

// Edit client dialog (old, removed)

const handleLogout = async () => {
  await logout()
  window.location.href = '/login'
}

const loadClients = async () => {
  try {
    clientsLoading.value = true
    clientsError.value = null
    const response = await fetch('/api/admin/clients', {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include',
    })

    if (!response.ok) {
      throw new Error('Failed to load clients')
    }

    const data = await response.json()
    clients.value = data
  } catch (error) {
    console.error('Failed to load clients:', error)
    clientsError.value = 'Failed to load clients'
  } finally {
    clientsLoading.value = false
  }
}

const loadDomains = async (clientId: string) => {
  try {
    domainsLoading.value = true
    domainsError.value = null
    const response = await fetch(`/api/admin/clients/${clientId}/domains`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include',
    })

    if (!response.ok) {
      throw new Error('Failed to load domains')
    }

    const data = await response.json()
    domains.value = data
  } catch (error) {
    console.error('Failed to load domains:', error)
    domainsError.value = 'Failed to load domains'
  } finally {
    domainsLoading.value = false
  }
}

const handleCreateClient = async () => {
  if (!newClientName.value.trim()) {
    return
  }

  createClientLoading.value = true
  try {
    const response = await fetch('/api/admin/clients', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include',
      body: JSON.stringify({
        name: newClientName.value.trim(),
        ai_api_key: newClientAiApiKey.value.trim() || null,
        ai_api_url: newClientAiApiUrl.value.trim() || null,
        ai_api_model: newClientAiApiModel.value.trim() || null,
      }),
    })

    if (!response.ok) {
      throw new Error('Failed to create client')
    }

    showCreateClient.value = false
    newClientName.value = ''
    newClientAiApiKey.value = ''
    newClientAiApiUrl.value = ''
    newClientAiApiModel.value = ''
    await loadClients()
  } catch (error) {
    console.error('Failed to create client:', error)
  } finally {
    createClientLoading.value = false
  }
}

const handleCreateDomain = async () => {
  if (!selectedClientForDomain.value || !newDomain.value.trim()) {
    return
  }

  createDomainLoading.value = true
  try {
    const response = await fetch(
      `/api/admin/clients/${selectedClientForDomain.value?.id}/domains`,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include',
        body: JSON.stringify({
          domain: newDomain.value.trim(),
        }),
      },
    )

    if (!response.ok) {
      throw new Error('Failed to add domain')
    }

    showCreateDomain.value = false
    newDomain.value = ''
    await loadDomains(selectedClientForDomain.value!.id)
  } catch (error) {
    console.error('Failed to add domain:', error)
  } finally {
    createDomainLoading.value = false
  }
}

// old handleEditClient removed

const handleUpdateClient = async () => {
  if (!editingClient.value || !editClientName.value.trim() || !editClientSlug.value.trim()) {
    return
  }

  editClientLoading.value = true
  try {
    const response = await fetch(`/api/admin/clients/${editingClient.value.id}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include',
      body: JSON.stringify({
        name: editClientName.value.trim(),
        slug: editClientSlug.value.trim(),
        ai_api_key: editClientAiApiKey.value.trim() || null,
        ai_api_url: editClientAiApiUrl.value.trim() || null,
        ai_api_model: editClientAiApiModel.value.trim() || null,
        is_active: editingClient.value.is_active,
      }),
    })

    if (!response.ok) {
      throw new Error('Failed to update client')
    }

    showEditModal.value = false
    editingClient.value = null
    editClientName.value = ''
    editClientSlug.value = ''
    editClientAiApiKey.value = ''
    editClientAiApiUrl.value = ''
    editClientAiApiModel.value = ''
    await loadClients()
  } catch (error) {
    console.error('Failed to update client:', error)
  } finally {
    editClientLoading.value = false
  }
}

const handleDeleteClient = async (client: Client) => {
  if (
    !confirm(
      `Are you sure you want to delete client "${client.name}"? This will also delete all associated domains and users.`,
    )
  ) {
    return
  }

  try {
    const response = await fetch(`/api/admin/clients/${client.id}`, {
      method: 'DELETE',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include',
    })

    if (!response.ok) {
      throw new Error('Failed to delete client')
    }

    await loadClients()
  } catch (error) {
    console.error('Failed to delete client:', error)
  }
}

const handleToggleDomainStatus = async (domain: Domain) => {
  try {
    const response = await fetch(`/api/admin/clients/${domain.client_id}/domains/${domain.id}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include',
      body: JSON.stringify({
        is_active: !domain.is_active,
      }),
    })

    if (!response.ok) {
      throw new Error('Failed to update domain status')
    }

    await loadDomains(domain.client_id)
    if (selectedClient.value && selectedClient.value.id === domain.client_id) {
      clientDomains.value = domains.value
    }
  } catch (error) {
    console.error('Failed to update domain status:', error)
  }
}

const handleDeleteDomain = async (domain: Domain) => {
  if (!confirm(`Are you sure you want to remove domain "${domain.domain}"?`)) {
    return
  }

  try {
    const response = await fetch(`/api/admin/clients/${domain.client_id}/domains/${domain.id}`, {
      method: 'DELETE',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include',
    })

    if (!response.ok) {
      throw new Error('Failed to remove domain')
    }

    await loadDomains(domain.client_id)
    if (selectedClient.value && selectedClient.value.id === domain.client_id) {
      clientDomains.value = domains.value
    }
  } catch (error) {
    console.error('Failed to remove domain:', error)
  }
}

const handleToggleClientStatus = async (client: Client) => {
  try {
    const response = await fetch(`/api/admin/clients/${client.id}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include',
      body: JSON.stringify({
        name: client.name,
        slug: client.slug,
        ai_api_key: client.ai_api_key,
        ai_api_url: client.ai_api_url,
        ai_api_model: client.ai_api_model,
        is_active: !client.is_active,
      }),
    })

    if (!response.ok) {
      throw new Error('Failed to update client status')
    }

    await loadClients()
  } catch (error) {
    console.error('Failed to update client status:', error)
  }
}

const handleViewClient = async (client: Client) => {
  selectedClient.value = client
  await loadDomains(client.id)
  clientDomains.value = domains.value
  showViewModal.value = true
}

const handleEditClient = (client: Client) => {
  editingClient.value = client
  editClientName.value = client.name
  editClientSlug.value = client.slug
  editClientAiApiKey.value = client.ai_api_key || ''
  editClientAiApiUrl.value = client.ai_api_url || ''
  editClientAiApiModel.value = client.ai_api_model || ''
  showEditModal.value = true
}

const handleAddDomainInModal = async () => {
  const domain = newDomainInModal.value.trim()
  if (!selectedClient.value || !domain || domain === '') {
    return
  }

  addDomainInModalLoading.value = true
  try {
    const response = await fetch(`/api/admin/clients/${selectedClient.value.id}/domains`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include',
      body: JSON.stringify({
        domain: domain,
      }),
    })

    if (!response.ok) {
      throw new Error('Failed to add domain')
    }

    newDomainInModal.value = ''
    await loadDomains(selectedClient.value.id)
    clientDomains.value = domains.value
  } catch (error) {
    console.error('Failed to add domain:', error)
  } finally {
    addDomainInModalLoading.value = false
  }
}

const handleSaveEditClient = async () => {
  if (!editingClient.value) return

  editClientLoading.value = true
  try {
    const response = await fetch(`/api/admin/clients/${editingClient.value.id}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include',
      body: JSON.stringify({
        name: editClientName.value,
        slug: editClientSlug.value,
        ai_api_key: editClientAiApiKey.value || null,
        ai_api_url: editClientAiApiUrl.value || null,
        ai_api_model: editClientAiApiModel.value || null,
        is_active: editingClient.value.is_active,
      }),
    })

    if (!response.ok) {
      throw new Error('Failed to update client')
    }

    showEditModal.value = false
    await loadClients()
  } catch (error) {
    console.error('Failed to update client:', error)
  } finally {
    editClientLoading.value = false
  }
}

const handleViewDomains = (client: Client) => {
  selectedClient.value = client
  loadDomains(client.id)
}

onMounted(async () => {
  await loadClients()
})
</script>

<template>
  <div class="container mx-auto p-8">
    <div class="flex justify-between items-center mb-8">
      <h1 class="text-3xl font-bold">Administrator Dashboard</h1>
      <Button @click="handleLogout" variant="outline">Logout</Button>
    </div>

    <div v-if="user" class="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Administrator Access</CardTitle>
          <CardDescription>Manage clients and domains for the multi-tenant system</CardDescription>
        </CardHeader>
        <CardContent>
          <div class="space-y-2">
            <p><strong>Username:</strong> {{ user.username }}</p>
            <p><strong>Role:</strong> Administrator</p>
             <p>
               <strong>Member since:</strong> {{ new Date(user.created_at).toLocaleDateString() }}
             </p>
           </div>
          </CardContent>
        </Card>

      <!-- Clients Section -->
      <Dialog v-model:open="showEditModal">
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit Client Settings</DialogTitle>
            <DialogDescription>
              Update the client details.
            </DialogDescription>
          </DialogHeader>
          <div class="space-y-4">
            <div>
              <Label for="edit-name">Name</Label>
              <Input id="edit-name" v-model="editClientName" />
            </div>
            <div>
              <Label for="edit-slug">Slug</Label>
              <Input id="edit-slug" v-model="editClientSlug" />
            </div>
            <div>
              <Label for="edit-ai-key">AI API Key</Label>
              <Input id="edit-ai-key" v-model="editClientAiApiKey" />
            </div>
            <div>
              <Label for="edit-ai-url">AI API URL</Label>
              <Input id="edit-ai-url" v-model="editClientAiApiUrl" />
            </div>
            <div>
              <Label for="edit-ai-model">AI API Model</Label>
              <Input id="edit-ai-model" v-model="editClientAiApiModel" />
            </div>
          </div>
          <DialogFooter>
            <Button @click="showEditModal = false" variant="outline">Cancel</Button>
            <Button @click="handleSaveEditClient" :disabled="editClientLoading">
              <span v-if="editClientLoading">Saving...</span>
              <span v-else>Save</span>
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

       <!-- Clients Section -->
       <div>
         <div class="flex justify-between items-center mb-6">
           <h2 class="text-2xl font-bold">Clients</h2>
           <Dialog v-model:open="showCreateClient">
             <DialogTrigger as-child>
               <Button>Create Client</Button>
             </DialogTrigger>
             <DialogContent>
               <DialogHeader>
                 <DialogTitle>Create New Client</DialogTitle>
                 <DialogDescription>
                   Add a new client to the multi-tenant system.
                 </DialogDescription>
               </DialogHeader>
               <div class="space-y-4">
                 <div>
                   <Label for="create-name">Name</Label>
                   <Input id="create-name" v-model="newClientName" placeholder="Client name" />
                 </div>
                 <div>
                   <Label for="create-ai-key">AI API Key (optional)</Label>
                   <Input id="create-ai-key" v-model="newClientAiApiKey" placeholder="API key" />
                 </div>
                 <div>
                   <Label for="create-ai-url">AI API URL (optional)</Label>
                   <Input id="create-ai-url" v-model="newClientAiApiUrl" placeholder="API URL" />
                 </div>
                 <div>
                   <Label for="create-ai-model">AI API Model (optional)</Label>
                   <Input id="create-ai-model" v-model="newClientAiApiModel" placeholder="Model name" />
                 </div>
               </div>
               <DialogFooter>
                 <Button @click="showCreateClient = false" variant="outline">Cancel</Button>
                 <Button @click="handleCreateClient" :disabled="createClientLoading || !newClientName.trim()">
                   <span v-if="createClientLoading">Creating...</span>
                   <span v-else>Create</span>
                 </Button>
               </DialogFooter>
             </DialogContent>
           </Dialog>
         </div>

         <div v-if="clientsLoading" class="text-center py-8">
           <p>Loading clients...</p>
         </div>

         <div v-else-if="clientsError" class="text-center py-8">
           <p class="text-red-600">{{ clientsError }}</p>
           <Button @click="loadClients" variant="outline" class="mt-2">Try Again</Button>
         </div>

         <div v-else-if="clients.length === 0" class="text-center py-8">
           <p class="text-gray-600">No clients found. Create your first client to get started.</p>
         </div>

         <div v-else class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
           <!-- Client Cards -->
           <Card
             v-for="client in clients"
             :key="client.id"
             class="group hover:shadow-md transition-shadow duration-200 h-full flex flex-col"
           >
             <CardHeader class="pb-3">
               <div class="flex items-start justify-between">
                 <div class="flex-1 min-w-0">
                   <div class="flex items-center space-x-2 mb-2">
                     <span
                       :class="client.is_active ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'"
                       class="px-2 py-1 rounded-full text-xs font-medium"
                     >
                       {{ client.is_active ? 'Active' : 'Inactive' }}
                     </span>
                     <span class="text-sm text-gray-600">Created {{ new Date(client.created_at).toLocaleDateString() }}</span>
                   </div>
                   <CardTitle class="text-lg font-semibold truncate">{{ client.name }}</CardTitle>
                   <CardDescription class="text-sm text-gray-500">{{ client.slug }}</CardDescription>
                 </div>

                 <div class="flex items-center space-x-2 ml-2 opacity-0 group-hover:opacity-100 transition-opacity duration-200">
                   <Button
                     @click="handleViewClient(client)"
                     variant="outline"
                     size="sm"
                     title="View Domains"
                   >
                     <Eye class="w-4 h-4" />
                   </Button>
                   <Button
                     @click="handleEditClient(client)"
                     variant="outline"
                     size="sm"
                     title="Edit Settings"
                   >
                     <Settings class="w-4 h-4" />
                   </Button>
                   <DropdownMenu>
                     <DropdownMenuTrigger as-child>
                       <Button variant="outline" size="sm">
                         <MoreVertical class="w-4 h-4" />
                       </Button>
                     </DropdownMenuTrigger>
                     <DropdownMenuContent>
                       <DropdownMenuItem @click="handleToggleClientStatus(client)">
                         {{ client.is_active ? 'Deactivate' : 'Activate' }} Client
                       </DropdownMenuItem>
                       <DropdownMenuItem @click="handleDeleteClient(client)" class="text-red-600 hover:text-red-700 hover:bg-red-50">
                         Delete Client
                       </DropdownMenuItem>
                     </DropdownMenuContent>
                   </DropdownMenu>
                 </div>
               </div>
             </CardHeader>

             <CardContent class="pt-0 flex-1">
               <div class="space-y-2 text-sm text-gray-600">
                 <div v-if="client.ai_api_key" class="flex items-center space-x-2">
                   <span class="font-medium">AI Key:</span>
                   <span class="truncate">{{ client.ai_api_key }}</span>
                 </div>
                 <div v-if="client.ai_api_url" class="flex items-center space-x-2">
                   <span class="font-medium">AI URL:</span>
                   <span class="truncate">{{ client.ai_api_url }}</span>
                 </div>
                 <div v-if="client.ai_api_model" class="flex items-center space-x-2">
                   <span class="font-medium">AI Model:</span>
                   <span class="truncate">{{ client.ai_api_model }}</span>
                 </div>
               </div>
             </CardContent>
           </Card>
         </div>
       </div>
     </div>

     <div v-else class="text-center">
       <p>Failed to load user profile</p>
     </div>

    <!-- View Domains Modal -->
    <Dialog v-model:open="showViewModal">
      <DialogContent class="max-w-4xl">
        <DialogHeader>
          <DialogTitle>Domains for {{ selectedClient?.name }}</DialogTitle>
          <DialogDescription> Manage domains for this client. </DialogDescription>
        </DialogHeader>
        <div class="space-y-4">
          <!-- Add Domain Form -->
          <div class="flex gap-2">
            <Input
              v-model="newDomainInModal"
              placeholder="Enter domain (e.g., example.com)"
              @keyup.enter="handleAddDomainInModal"
            />
            <Button
              @click="handleAddDomainInModal"
              :disabled="addDomainInModalLoading || !newDomainInModal.trim()"
            >
              <span v-if="addDomainInModalLoading">Adding...</span>
              <span v-else>Add Domain</span>
            </Button>
          </div>

          <!-- Domains Table -->
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Domain</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-if="clientDomains.length === 0">
                <TableCell colspan="3" class="text-center py-8 text-gray-600">
                  No domains configured for this client.
                </TableCell>
              </TableRow>
              <TableRow v-else v-for="domain in clientDomains" :key="domain.id">
                <TableCell>{{ domain.domain }}</TableCell>
                <TableCell>
                  <span
                    :class="domain.is_active ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'"
                    class="px-2 py-1 rounded-full text-xs font-medium"
                  >
                    {{ domain.is_active ? 'Active' : 'Inactive' }}
                  </span>
                </TableCell>
                <TableCell>
                  <div class="flex gap-2">
                    <Button
                      :variant="domain.is_active ? 'destructive' : 'default'"
                      size="sm"
                      @click="handleToggleDomainStatus(domain)"
                    >
                      {{ domain.is_active ? 'Deactivate' : 'Activate' }}
                    </Button>
                    <AlertDialog>
                      <AlertDialogTrigger as-child>
                        <Button variant="destructive" size="sm">Delete</Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>Delete Domain</AlertDialogTitle>
                          <AlertDialogDescription>
                            Are you sure you want to delete "{{ domain.domain }}"? This action cannot be undone.
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>Cancel</AlertDialogCancel>
                          <AlertDialogAction @click="handleDeleteDomain(domain)">Delete</AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  </div>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
      </DialogContent>
     </Dialog>
   </div>
</template>
