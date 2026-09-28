import { afterEach, describe, expect, it } from 'vitest'
import { useEnvironmentsStore } from './environments'

afterEach(() => {
  useEnvironmentsStore.setState({ environments: [], activeEnvId: null, extensionVariables: {} })
})

describe('extension variable providers', () => {
  it('keeps provider values in memory and lets the active environment override collisions', () => {
    useEnvironmentsStore.setState({
      extensionVariables: { base_url: 'https://provider.test', provider_only: 'yes' },
      activeEnvId: 'env-1',
      environments: [{
        id: 'env-1',
        name: 'Local',
        variables: [
          { id: 'var-1', key: 'base_url', value: 'https://local.test', enabled: true },
          { id: 'var-2', key: 'disabled', value: 'no', enabled: false },
        ],
      }],
    })

    expect(useEnvironmentsStore.getState().getResolvedVars()).toEqual({
      base_url: 'https://local.test',
      provider_only: 'yes',
    })
  })
})
