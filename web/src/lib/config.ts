export interface SystemConfig {
  beta_mode: boolean
  registration_enabled?: boolean
  initialized?: boolean
}

let configPromise: Promise<SystemConfig> | null = null
let cachedConfig: SystemConfig | null = null

export function getSystemConfig(): Promise<SystemConfig> {
  if (cachedConfig) {
    return Promise.resolve(cachedConfig)
  }
  if (configPromise) {
    return configPromise
  }
  configPromise = fetch('/api/config')
    .then((res) => {
      if (!res.ok) {
        throw new Error(
          `Failed to fetch config: ${res.status} ${res.statusText}`
        )
      }
      const contentType = res.headers.get('content-type')
      if (!contentType || !contentType.includes('application/json')) {
        throw new Error('Expected JSON response but received something else')
      }
      return res.json()
    })
    .then((data: SystemConfig) => {
      cachedConfig = data
      return data
    })
    .finally(() => {
      // Keep cachedConfig for reuse; allow re-fetch via explicit invalidation if added later
    })
  return configPromise
}
