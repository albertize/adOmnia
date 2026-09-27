import { create } from 'zustand'
import {
  disableExtension,
  enableExtension,
  executeExtensionCommand,
  installExtensionArchive,
  installExtensionDirectory,
  listExtensions,
  reloadExtension,
  setExtensionGrants,
  uninstallExtension,
  type ExtensionInstance,
  type HostExecutionResult,
} from '@/lib/extensions-v2-api'

interface ExtensionsState {
  extensions: ExtensionInstance[]
  loading: boolean
  error: string | null
  load: () => Promise<void>
  installDirectory: (path: string, development?: boolean) => Promise<void>
  installArchive: (path: string) => Promise<void>
  reloadSource: (id: string) => Promise<void>
  setGrants: (id: string, grants: string[]) => Promise<void>
  enable: (id: string) => Promise<void>
  disable: (id: string) => Promise<void>
  uninstall: (id: string) => Promise<void>
  execute: (extensionId: string, commandId: string, source?: string) => Promise<HostExecutionResult>
}

function message(error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}

export const useExtensionsStore = create<ExtensionsState>((set, get) => ({
  extensions: [],
  loading: false,
  error: null,
  load: async () => {
    set({ loading: true, error: null })
    try {
      set({ extensions: await listExtensions() })
    } catch (error) {
      set({ error: message(error) })
    } finally {
      set({ loading: false })
    }
  },
  installDirectory: async (path, development = false) => {
    set({ loading: true, error: null })
    try {
      await installExtensionDirectory(path, development)
      await get().load()
    } catch (error) {
      set({ error: message(error), loading: false })
      throw error
    }
  },
  installArchive: async (path) => {
    set({ loading: true, error: null })
    try {
      await installExtensionArchive(path)
      await get().load()
    } catch (error) {
      set({ error: message(error), loading: false })
      throw error
    }
  },
  reloadSource: async (id) => {
    set({ error: null })
    try {
      await reloadExtension(id)
      await get().load()
    } catch (error) {
      set({ error: message(error) })
      throw error
    }
  },
  setGrants: async (id, grants) => {
    set({ error: null })
    try {
      await setExtensionGrants(id, grants)
      await get().load()
    } catch (error) {
      set({ error: message(error) })
      throw error
    }
  },
  enable: async (id) => {
    set({ error: null })
    try {
      await enableExtension(id)
      await get().load()
    } catch (error) {
      set({ error: message(error) })
      throw error
    }
  },
  disable: async (id) => {
    set({ error: null })
    try {
      await disableExtension(id)
      await get().load()
    } catch (error) {
      set({ error: message(error) })
      throw error
    }
  },
  uninstall: async (id) => {
    set({ error: null })
    try {
      await uninstallExtension(id)
      await get().load()
    } catch (error) {
      set({ error: message(error) })
      throw error
    }
  },
  execute: (extensionId, commandId, source = 'extension') => executeExtensionCommand(extensionId, commandId, {}, source),
}))
