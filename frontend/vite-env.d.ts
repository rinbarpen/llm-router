/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_PORT?: string
  readonly VITE_API_URL?: string
  readonly VITE_API_BASE_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv & {
    readonly DEV: boolean
    readonly MODE: string
    readonly PROD: boolean
    readonly SSR: boolean
  }
}

declare module 'vite/client' {
  interface ImportMetaEnv {
    readonly VITE_PORT?: string
    readonly VITE_API_URL?: string
    readonly VITE_API_BASE_URL?: string
  }
}

