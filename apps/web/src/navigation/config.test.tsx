import { describe, expect, it } from 'vitest'
import { navItems } from './config'

describe('cubebox navigation config', () => {
  it('uses cubebox-specific labels and aligns models permission with route access', () => {
    const cubebox = navItems.find((item) => item.key === 'cubebox')
    const cubeboxModels = navItems.find((item) => item.key === 'cubebox-models')
    const cubeboxFiles = navItems.find((item) => item.key === 'cubebox-files')

    expect(cubebox?.labelKey).toBe('nav_cubebox')
    expect(cubeboxModels?.labelKey).toBe('nav_cubebox_models')
    expect(cubeboxFiles?.labelKey).toBe('nav_cubebox_files')
    expect(cubeboxModels?.permissionKey).toBe('orgunit.read')
  })
})
