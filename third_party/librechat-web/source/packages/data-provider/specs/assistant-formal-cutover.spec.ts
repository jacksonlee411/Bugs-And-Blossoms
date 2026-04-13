type RequestDouble = {
  get: jest.Mock;
  post: jest.Mock;
  put: jest.Mock;
  patch: jest.Mock;
  delete: jest.Mock;
};

const bootstrapPayload = {
  contract_version: 'v1',
  viewer: {
    id: 'principal_1',
    username: 'tenant-admin',
    email: 'tenant-admin@example.invalid',
    name: 'Tenant Admin',
    role: 'USER',
  },
  ui: {
    model_select: true,
    artifacts_enabled: true,
    agents_ui_enabled: false,
    memory_enabled: false,
    web_search_enabled: false,
    file_search_enabled: false,
    code_interpreter_enabled: false,
  },
  models: [
    {
      endpoint_key: 'assistantFormal',
      endpoint_type: 'openAI',
      provider: 'openai',
      model: 'gpt-5.4',
      label: 'GPT-5.4',
    },
  ],
  runtime: {
    status: 'healthy',
    runtime_cutover_mode: 'ui-shell-only',
    domain_policy_version: 'v1',
  },
};

const sessionPayload = {
  contract_version: 'v1',
  authenticated: true,
  viewer: {
    id: 'principal_1',
    username: 'tenant-admin',
    email: 'tenant-admin@example.invalid',
    name: 'Tenant Admin',
    role: 'USER',
  },
};

function createRequestDouble(): RequestDouble {
  return {
    get: jest.fn(),
    post: jest.fn(),
    put: jest.fn(),
    patch: jest.fn(),
    delete: jest.fn(),
  };
}

async function loadDataService(requestDouble: RequestDouble) {
  jest.resetModules();
  jest.doMock('../src/request', () => ({
    __esModule: true,
    default: requestDouble,
  }));
  return await import('../src/data-service');
}

describe('assistant formal cutover data service', () => {
  afterEach(() => {
    jest.resetModules();
    jest.clearAllMocks();
  });

  it('uses successor bootstrap for startup config, endpoints and models', async () => {
    const requestDouble = createRequestDouble();
    requestDouble.get.mockResolvedValue(bootstrapPayload);

    const dataService = await loadDataService(requestDouble);
    const startupConfig = await dataService.getStartupConfig();
    const endpointsConfig = await dataService.getAIEndpoints();
    const modelsConfig = await dataService.getModels();

    expect(requestDouble.get).toHaveBeenCalledTimes(1);
    expect(requestDouble.get).toHaveBeenCalledWith('/internal/assistant/ui-bootstrap');
    expect(startupConfig.appTitle).toBe('Bugs and Blossoms Assistant');
    expect(startupConfig.interface.agents).toBe(false);
    expect(startupConfig.interface.memories).toBe(false);
    expect(startupConfig.interface.webSearch).toBe(false);
    expect(startupConfig.interface.fileSearch).toBe(false);
    expect(startupConfig.interface.runCode).toBe(false);
    expect(startupConfig.interface.marketplace?.use).toBe(false);
    expect(startupConfig.modelSpecs?.addedEndpoints).toEqual(['assistantFormal']);
    expect(endpointsConfig).toMatchObject({
      assistantFormal: {
        name: 'Assistant',
        disableBuilder: true,
        userProvide: false,
      },
    });
    expect(modelsConfig).toEqual({
      assistantFormal: ['GPT-5.4'],
    });
  });

  it('uses successor session endpoints for viewer and logout', async () => {
    const requestDouble = createRequestDouble();
    requestDouble.get.mockResolvedValue(sessionPayload);
    requestDouble.post.mockResolvedValue(undefined);

    const dataService = await loadDataService(requestDouble);
    const viewer = await dataService.getUser();
    const logoutResponse = await dataService.logout();

    expect(requestDouble.get).toHaveBeenCalledWith('/internal/assistant/session');
    expect(requestDouble.post).toHaveBeenCalledWith('/internal/assistant/session/logout');
    expect(viewer).toMatchObject({
      id: 'principal_1',
      username: 'tenant-admin',
      email: 'tenant-admin@example.invalid',
      provider: 'bugs-and-blossoms-sid',
    });
    expect(logoutResponse).toEqual({
      message: 'Logged out',
      redirect: '/app/login',
    });
  });
});
