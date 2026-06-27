import {
  keepPreviousData,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import {
  BarChart3,
  Bot,
  Brain,
  Check,
  Copy,
  ExternalLink,
  Eye,
  EyeOff,
  X as IconX,
  Landmark,
  Pencil,
  Plus,
  Trash2,
  Users,
} from 'lucide-react'
import React, { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { t, type Language } from '../i18n/translations'
import { api } from '../lib/api'
import { confirmToast } from '../lib/notify'
import type {
  AIModel,
  CreateTraderRequest,
  Exchange,
  TraderInfo,
} from '../types'
import { DeepVoidBackground } from './DeepVoidBackground'
import { getExchangeIcon } from './ExchangeIcons'
import { getModelIcon } from './ModelIcons'
import { PunkAvatar, getTraderAvatar } from './PunkAvatar'
import { TraderConfigModal } from './TraderConfigModal'
import { ExchangeConfigModal } from './traders/ExchangeConfigModal'
import { CustomSelect } from './ui/CustomSelect'

// 获取友好的AI模型名称
function getModelDisplayName(modelId: string): string {
  switch (modelId.toLowerCase()) {
    case 'deepseek':
      return 'DeepSeek'
    case 'qwen':
      return 'Qwen'
    case 'claude':
      return 'Claude'
    default:
      return modelId.toUpperCase()
  }
}

// 提取下划线后面的名称部分
function getShortName(fullName: string): string {
  const parts = fullName.split('_')
  return parts.length > 1 ? parts[parts.length - 1] : fullName
}

// AI Provider configuration - default models and API links
const AI_PROVIDER_CONFIG: Record<
  string,
  {
    defaultModel: string
    apiUrl: string
    apiName: string
  }
> = {
  deepseek: {
    defaultModel: 'deepseek-chat',
    apiUrl: 'https://platform.deepseek.com/api_keys',
    apiName: 'DeepSeek',
  },
  qwen: {
    defaultModel: 'qwen3-max',
    apiUrl: 'https://dashscope.console.aliyun.com/apiKey',
    apiName: 'Alibaba Cloud',
  },
  openai: {
    defaultModel: 'gpt-5.2',
    apiUrl: 'https://platform.openai.com/api-keys',
    apiName: 'OpenAI',
  },
  claude: {
    defaultModel: 'claude-opus-4-5-20251101',
    apiUrl: 'https://console.anthropic.com/settings/keys',
    apiName: 'Anthropic',
  },
  gemini: {
    defaultModel: 'gemini-3-pro-preview',
    apiUrl: 'https://aistudio.google.com/app/apikey',
    apiName: 'Google AI Studio',
  },
  grok: {
    defaultModel: 'grok-3-latest',
    apiUrl: 'https://console.x.ai/',
    apiName: 'xAI',
  },
  kimi: {
    defaultModel: 'moonshot-v1-auto',
    apiUrl: 'https://platform.moonshot.ai/console/api-keys',
    apiName: 'Moonshot',
  },
}

interface AITradersPageProps {
  onTraderSelect?: (traderId: string) => void
}

// Helper function to get exchange display name from exchange ID (UUID)
function getExchangeDisplayName(
  exchangeId: string | undefined,
  exchanges: Exchange[]
): string {
  if (!exchangeId) return 'Unknown'
  const exchange = exchanges.find((e) => e.id === exchangeId)
  if (!exchange) return exchangeId.substring(0, 8).toUpperCase() + '...' // Show truncated UUID if not found
  const typeName = exchange.exchange_type?.toUpperCase() || exchange.name
  return exchange.account_name
    ? `${typeName} - ${exchange.account_name}`
    : typeName
}

// Helper function to check if exchange is a perp-dex type (wallet-based)
function isPerpDexExchange(exchangeType: string | undefined): boolean {
  if (!exchangeType) return false
  const perpDexTypes = ['hyperliquid', 'lighter', 'aster']
  return perpDexTypes.includes(exchangeType.toLowerCase())
}

// Helper function to get wallet address for perp-dex exchanges
function getWalletAddress(exchange: Exchange | undefined): string | undefined {
  if (!exchange) return undefined
  const type = exchange.exchange_type?.toLowerCase()
  switch (type) {
    case 'hyperliquid':
      return exchange.hyperliquidWalletAddr
    case 'lighter':
      return exchange.lighterWalletAddr
    case 'aster':
      return exchange.asterSigner
    default:
      return undefined
  }
}

// Helper function to truncate wallet address for display
function truncateAddress(address: string, startLen = 6, endLen = 4): string {
  if (address.length <= startLen + endLen + 3) return address
  return `${address.slice(0, startLen)}...${address.slice(-endLen)}`
}

export function AITradersPage({ onTraderSelect }: AITradersPageProps) {
  const { language } = useLanguage()
  const { user, token } = useAuth()
  const navigate = useNavigate()
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [showEditModal, setShowEditModal] = useState(false)
  const [showModelModal, setShowModelModal] = useState(false)
  const [showExchangeModal, setShowExchangeModal] = useState(false)
  const [editingModel, setEditingModel] = useState<string | null>(null)
  const [editingExchange, setEditingExchange] = useState<string | null>(null)
  const [editingTrader, setEditingTrader] = useState<any>(null)
  const [allModels, setAllModels] = useState<AIModel[]>([])
  const [allExchanges, setAllExchanges] = useState<Exchange[]>([])
  const [supportedModels, setSupportedModels] = useState<AIModel[]>([])
  const [visibleTraderAddresses, setVisibleTraderAddresses] = useState<
    Set<string>
  >(new Set())
  const [visibleExchangeAddresses, setVisibleExchangeAddresses] = useState<
    Set<string>
  >(new Set())
  const [copiedId, setCopiedId] = useState<string | null>(null)
  // Track loading state for individual trader toggle operations
  const [togglingTraders, setTogglingTraders] = useState<Set<string>>(new Set())

  // Toggle wallet address visibility for a trader
  const toggleTraderAddressVisibility = (traderId: string) => {
    setVisibleTraderAddresses((prev) => {
      const next = new Set(prev)
      if (next.has(traderId)) {
        next.delete(traderId)
      } else {
        next.add(traderId)
      }
      return next
    })
  }

  // Toggle wallet address visibility for an exchange
  const toggleExchangeAddressVisibility = (exchangeId: string) => {
    setVisibleExchangeAddresses((prev) => {
      const next = new Set(prev)
      if (next.has(exchangeId)) {
        next.delete(exchangeId)
      } else {
        next.add(exchangeId)
      }
      return next
    })
  }

  // Copy wallet address to clipboard
  const handleCopyAddress = async (id: string, address: string) => {
    try {
      await navigator.clipboard.writeText(address)
      setCopiedId(id)
      setTimeout(() => setCopiedId(null), 2000)
    } catch (err) {
      console.error('Failed to copy address:', err)
    }
  }

  const queryClient = useQueryClient()
  const { data: traders, isLoading: isTradersLoading } = useQuery<TraderInfo[]>(
    {
      queryKey: ['traders'],
      queryFn: api.getTraders,
      enabled: !!(user && token),
      staleTime: 5000,
      refetchInterval: 10000,
      placeholderData: keepPreviousData,
    }
  )

  const mutateTraders = () =>
    queryClient.invalidateQueries({ queryKey: ['traders'] })

  // 加载AI模型和交易所配置
  useEffect(() => {
    const loadConfigs = async () => {
      if (!user || !token) {
        // 未登录时只加载公开的支持模型
        try {
          const supportedModels = await api.getSupportedModels()
          setSupportedModels(supportedModels)
        } catch (err) {
          console.error('Failed to load supported configs:', err)
        }
        return
      }

      try {
        const [modelConfigs, exchangeConfigs, supportedModels] =
          await Promise.all([
            api.getModelConfigs(),
            api.getExchangeConfigs(),
            api.getSupportedModels(),
          ])
        setAllModels(modelConfigs)
        setAllExchanges(exchangeConfigs)
        setSupportedModels(supportedModels)
      } catch (error) {
        console.error('Failed to load configs:', error)
      }
    }
    loadConfigs()
  }, [user, token])

  // 只显示已配置的模型和交易所
  // 注意：后端返回的数据不包含敏感信息（apiKey等），所以通过其他字段判断是否已配置
  const configuredModels =
    allModels?.filter((m) => {
      // 如果模型已启用，说明已配置
      // 或者有自定义API URL，也说明已配置
      return m.enabled || (m.customApiUrl && m.customApiUrl.trim() !== '')
    }) || []
  const configuredExchanges =
    allExchanges?.filter((e) => {
      // Aster 交易所检查特殊字段
      if (e.id === 'aster') {
        return e.asterUser && e.asterUser.trim() !== ''
      }
      // Hyperliquid 需要检查钱包地址（后端会返回这个字段）
      if (e.id === 'hyperliquid') {
        return e.hyperliquidWalletAddr && e.hyperliquidWalletAddr.trim() !== ''
      }
      // 其他交易所：如果已启用，说明已配置（后端返回的已配置交易所会有 enabled: true）
      return e.enabled
    }) || []

  // 只在创建交易员时使用已启用且配置完整的
  // 注意：后端返回的数据不包含敏感信息，所以只检查 enabled 状态和必要的非敏感字段
  const enabledModels = allModels?.filter((m) => m.enabled) || []
  const enabledExchanges =
    allExchanges?.filter((e) => {
      if (!e.enabled) return false

      // Aster 交易所需要特殊字段（后端会返回这些非敏感字段）
      if (e.id === 'aster') {
        return (
          e.asterUser &&
          e.asterUser.trim() !== '' &&
          e.asterSigner &&
          e.asterSigner.trim() !== ''
        )
      }

      // Hyperliquid 需要钱包地址（后端会返回这个字段）
      if (e.id === 'hyperliquid') {
        return e.hyperliquidWalletAddr && e.hyperliquidWalletAddr.trim() !== ''
      }

      // 其他交易所：如果已启用，说明已配置完整（后端只返回已配置的交易所）
      return true
    }) || []

  // 检查模型是否正在被运行中的交易员使用（用于UI禁用）
  const isModelInUse = (modelId: string) => {
    return traders?.some((t) => t.ai_model === modelId && t.is_running)
  }

  // 检查模型被哪些交易员使用
  const getModelUsageInfo = (modelId: string) => {
    const usingTraders = traders?.filter((t) => t.ai_model === modelId) || []
    const runningCount = usingTraders.filter((t) => t.is_running).length
    const totalCount = usingTraders.length
    return { runningCount, totalCount, usingTraders }
  }

  // 检查交易所是否正在被运行中的交易员使用（用于UI禁用）
  const isExchangeInUse = (exchangeId: string) => {
    return traders?.some((t) => t.exchange_id === exchangeId && t.is_running)
  }

  // 检查交易所被哪些交易员使用
  const getExchangeUsageInfo = (exchangeId: string) => {
    const usingTraders =
      traders?.filter((t) => t.exchange_id === exchangeId) || []
    const runningCount = usingTraders.filter((t) => t.is_running).length
    const totalCount = usingTraders.length
    return { runningCount, totalCount, usingTraders }
  }

  // 检查模型是否被任何交易员使用（包括停止状态的）
  const isModelUsedByAnyTrader = (modelId: string) => {
    return traders?.some((t) => t.ai_model === modelId) || false
  }

  // 检查交易所是否被任何交易员使用（包括停止状态的）
  const isExchangeUsedByAnyTrader = (exchangeId: string) => {
    return traders?.some((t) => t.exchange_id === exchangeId) || false
  }

  // 获取使用特定模型的交易员列表
  const getTradersUsingModel = (modelId: string) => {
    return traders?.filter((t) => t.ai_model === modelId) || []
  }

  // 获取使用特定交易所的交易员列表
  const getTradersUsingExchange = (exchangeId: string) => {
    return traders?.filter((t) => t.exchange_id === exchangeId) || []
  }

  const handleCreateTrader = async (data: CreateTraderRequest) => {
    try {
      const model = allModels?.find((m) => m.id === data.ai_model_id)
      const exchange = allExchanges?.find((e) => e.id === data.exchange_id)

      if (!model?.enabled) {
        toast.error('Model not configured')
        return
      }

      if (!exchange?.enabled) {
        toast.error('Exchange not configured')
        return
      }

      await toast.promise(api.createTrader(data), {
        loading: 'Creating...',
        success: 'Created successfully',
        error: 'Failed to create',
      })
      setShowCreateModal(false)
      // Immediately refresh traders list for better UX
      await mutateTraders()
    } catch (error) {
      console.error('Failed to create trader:', error)
      toast.error('Failed to create trader')
    }
  }

  const handleEditTrader = async (traderId: string) => {
    try {
      const traderConfig = await api.getTraderConfig(traderId)
      setEditingTrader(traderConfig)
      setShowEditModal(true)
    } catch (error) {
      console.error('Failed to fetch trader config:', error)
      toast.error('Failed to fetch trader config')
    }
  }

  const handleSaveEditTrader = async (data: CreateTraderRequest) => {
    console.log('🔥🔥🔥 handleSaveEditTrader CALLED with data:', data)
    if (!editingTrader) return

    try {
      const model = enabledModels?.find((m) => m.id === data.ai_model_id)
      const exchange = enabledExchanges?.find((e) => e.id === data.exchange_id)

      if (!model) {
        toast.error('Model configuration does not exist or is not enabled')
        return
      }

      if (!exchange) {
        toast.error('Exchange configuration does not exist or is not enabled')
        return
      }

      const request = {
        name: data.name,
        ai_model_id: data.ai_model_id,
        exchange_id: data.exchange_id,
        strategy_id: data.strategy_id,
        initial_balance: data.initial_balance,
        scan_interval_minutes: data.scan_interval_minutes,
        is_cross_margin: data.is_cross_margin,
        show_in_competition: data.show_in_competition,
      }

      console.log('🔥 handleSaveEditTrader - data:', data)
      console.log(
        '🔥 handleSaveEditTrader - data.strategy_id:',
        data.strategy_id
      )
      console.log('🔥 handleSaveEditTrader - request:', request)

      await toast.promise(api.updateTrader(editingTrader.trader_id, request), {
        loading: 'Saving...',
        success: 'Saved successfully',
        error: 'Failed to save',
      })
      setShowEditModal(false)
      setEditingTrader(null)
      // Immediately refresh traders list for better UX
      await mutateTraders()
    } catch (error) {
      console.error('Failed to update trader:', error)
      toast.error('Failed to update trader')
    }
  }

  const handleDeleteTrader = async (traderId: string) => {
    {
      const ok = await confirmToast('Confirm delete trader')
      if (!ok) return
    }

    try {
      await toast.promise(api.deleteTrader(traderId), {
        loading: 'Deleting...',
        success: 'Deleted successfully',
        error: 'Failed to delete',
      })

      // Immediately refresh traders list for better UX
      await mutateTraders()
    } catch (error) {
      console.error('Failed to delete trader:', error)
      toast.error('Failed to delete trader')
    }
  }

  const handleToggleTrader = async (traderId: string, running: boolean) => {
    // Mark this trader as toggling (for loading indicator)
    setTogglingTraders((prev) => new Set(prev).add(traderId))

    // Optimistic update - update cache immediately for instant UI feedback
    const previousTraders = queryClient.getQueryData<TraderInfo[]>(['traders'])
    queryClient.setQueryData<TraderInfo[]>(['traders'], (old) =>
      old?.map((t) =>
        t.trader_id === traderId ? { ...t, is_running: !running } : t
      )
    )

    try {
      if (running) {
        await api.stopTrader(traderId)
        toast.success('Stopped')
      } else {
        await api.startTrader(traderId)
        toast.success('Started')
      }

      // Refresh to get actual state from server (in background)
      mutateTraders()
      
      // Also invalidate all trader-specific data to ensure Dashboard is instantly responsive
      queryClient.invalidateQueries({ queryKey: ['positions', traderId] })
      queryClient.invalidateQueries({ queryKey: ['account', traderId] })
      queryClient.invalidateQueries({ queryKey: ['status', traderId] })
      queryClient.invalidateQueries({ queryKey: ['decisions', traderId] })
      queryClient.invalidateQueries({ queryKey: ['statistics', traderId] })
      queryClient.invalidateQueries({ queryKey: ['position-history', traderId] })
    } catch (error) {
      // Rollback optimistic update on error
      queryClient.setQueryData(['traders'], previousTraders)
      console.error('Failed to toggle trader:', error)
      toast.error('Failed to toggle trader')
    } finally {
      // Remove from toggling set
      setTogglingTraders((prev) => {
        const next = new Set(prev)
        next.delete(traderId)
        return next
      })
    }
  }

  const handleToggleCompetition = async (
    traderId: string,
    currentShowInCompetition: boolean
  ) => {
    try {
      const newValue = !currentShowInCompetition
      await toast.promise(api.toggleCompetition(traderId, newValue), {
        loading: 'Updating...',
        success: newValue ? 'Now visible in arena' : 'Hidden from arena',
        error: 'Failed to update',
      })

      // Immediately refresh traders list to update status
      await mutateTraders()
    } catch (error) {
      console.error('Failed to toggle competition visibility:', error)
      toast.error('Failed to toggle competition visibility')
    }
  }

  const handleModelClick = (modelId: string) => {
    if (!isModelInUse(modelId)) {
      setEditingModel(modelId)
      setShowModelModal(true)
    }
  }

  const handleExchangeClick = (exchangeId: string) => {
    if (!isExchangeInUse(exchangeId)) {
      setEditingExchange(exchangeId)
      setShowExchangeModal(true)
    }
  }

  // 通用删除配置处理函数
  const handleDeleteConfig = async <T extends { id: string }>(config: {
    id: string
    type: 'model' | 'exchange'
    checkInUse: (id: string) => boolean
    getUsingTraders: (id: string) => any[]
    cannotDeleteKey: string
    confirmDeleteKey: string
    allItems: T[] | undefined
    clearFields: (item: T) => T
    buildRequest: (items: T[]) => any
    updateApi: (request: any) => Promise<void>
    refreshApi: () => Promise<T[]>
    setItems: (items: T[]) => void
    closeModal: () => void
    errorKey: string
  }) => {
    // 检查是否有交易员正在使用
    if (config.checkInUse(config.id)) {
      const usingTraders = config.getUsingTraders(config.id)
      const traderNames = usingTraders.map((t) => t.trader_name).join(', ')
      toast.error(
        `${config.cannotDeleteKey} · Traders using: ${traderNames} · Please delete traders first`
      )
      return
    }

    {
      const ok = await confirmToast('Confirm delete ' + config.type)
      if (!ok) return
    }

    try {
      const updatedItems =
        config.allItems?.map((item) =>
          item.id === config.id ? config.clearFields(item) : item
        ) || []

      const request = config.buildRequest(updatedItems)
      await toast.promise(config.updateApi(request), {
        loading: 'Updating config...',
        success: 'Config updated',
        error: 'Failed to update config',
      })

      const refreshedItems = await config.refreshApi()
      config.setItems(refreshedItems)

      config.closeModal()
    } catch (error) {
      console.error(`Failed to delete ${config.type} config:`, error)
      toast.error('Failed to delete ' + config.type + ' config')
    }
  }

  const handleDeleteModelConfig = async (modelId: string) => {
    await handleDeleteConfig({
      id: modelId,
      type: 'model',
      checkInUse: isModelUsedByAnyTrader,
      getUsingTraders: getTradersUsingModel,
      cannotDeleteKey: 'cannotDeleteModelInUse',
      confirmDeleteKey: 'confirmDeleteModel',
      allItems: allModels,
      clearFields: (m) => ({
        ...m,
        apiKey: '',
        customApiUrl: '',
        customModelName: '',
        enabled: false,
      }),
      buildRequest: (models) => ({
        models: Object.fromEntries(
          models.map((model) => [
            model.provider,
            {
              enabled: model.enabled,
              api_key: model.apiKey || '',
              custom_api_url: model.customApiUrl || '',
              custom_model_name: model.customModelName || '',
            },
          ])
        ),
      }),
      updateApi: api.updateModelConfigs,
      refreshApi: api.getModelConfigs,
      setItems: (items) => {
        // 使用函数式更新确保状态正确更新
        setAllModels([...items])
      },
      closeModal: () => {
        setShowModelModal(false)
        setEditingModel(null)
      },
      errorKey: 'deleteConfigFailed',
    })
  }

  const handleSaveModelConfig = async (
    modelId: string,
    apiKey: string,
    customApiUrl?: string,
    customModelName?: string
  ) => {
    try {
      // 创建或更新用户的模型配置
      const existingModel = allModels?.find((m) => m.id === modelId)
      let updatedModels

      // 找到要配置的模型（优先从已配置列表，其次从支持列表）
      const modelToUpdate =
        existingModel || supportedModels?.find((m) => m.id === modelId)
      if (!modelToUpdate) {
        toast.error('Model not exist')
        return
      }

      if (existingModel) {
        // 更新现有配置
        updatedModels =
          allModels?.map((m) =>
            m.id === modelId
              ? {
                  ...m,
                  apiKey,
                  customApiUrl: customApiUrl || '',
                  customModelName: customModelName || '',
                  enabled: true,
                }
              : m
          ) || []
      } else {
        // 添加新配置
        const newModel = {
          ...modelToUpdate,
          apiKey,
          customApiUrl: customApiUrl || '',
          customModelName: customModelName || '',
          enabled: true,
        }
        updatedModels = [...(allModels || []), newModel]
      }

      const request = {
        models: Object.fromEntries(
          updatedModels.map((model) => [
            model.provider, // 使用 provider 而不是 id
            {
              enabled: model.enabled,
              api_key: model.apiKey || '',
              custom_api_url: model.customApiUrl || '',
              custom_model_name: model.customModelName || '',
            },
          ])
        ),
      }

      await toast.promise(api.updateModelConfigs(request), {
        loading: 'Updating model config...',
        success: 'Model config updated',
        error: 'Failed to update model config',
      })

      // 重新获取用户配置以确保数据同步
      const refreshedModels = await api.getModelConfigs()
      setAllModels(refreshedModels)

      setShowModelModal(false)
      setEditingModel(null)
    } catch (error) {
      console.error('Failed to save model config:', error)
      toast.error('Failed to save model config')
    }
  }

  const handleDeleteExchangeConfig = async (exchangeId: string) => {
    // 检查是否有trader在使用此交易所账户
    if (isExchangeUsedByAnyTrader(exchangeId)) {
      const tradersUsing = getTradersUsingExchange(exchangeId)
      toast.error(`Cannot delete exchange in use: ${tradersUsing.join(', ')}`)
      return
    }

    const ok = await confirmToast('Confirm delete exchange')
    if (!ok) return

    try {
      await toast.promise(api.deleteExchange(exchangeId), {
        loading: 'Deleting exchange account...',
        success: 'Exchange account deleted',
        error: 'Failed to delete exchange account',
      })

      // 重新获取用户配置以确保数据同步
      const refreshedExchanges = await api.getExchangeConfigs()
      setAllExchanges(refreshedExchanges)

      setShowExchangeModal(false)
      setEditingExchange(null)
    } catch (error) {
      console.error('Failed to delete exchange config:', error)
      toast.error('Failed to delete exchange config')
    }
  }

  const handleSaveExchangeConfig = async (
    exchangeId: string | null, // null for creating new account
    exchangeType: string,
    accountName: string,
    apiKey: string,
    secretKey?: string,
    passphrase?: string,
    testnet?: boolean,
    hyperliquidWalletAddr?: string,
    asterUser?: string,
    asterSigner?: string,
    asterPrivateKey?: string,
    lighterWalletAddr?: string,
    lighterPrivateKey?: string,
    lighterApiKeyPrivateKey?: string,
    lighterApiKeyIndex?: number
  ) => {
    try {
      if (exchangeId) {
        // 更新现有账户配置
        const existingExchange = allExchanges?.find((e) => e.id === exchangeId)
        if (!existingExchange) {
          toast.error('Exchange not exist')
          return
        }

        const request = {
          exchanges: {
            [exchangeId]: {
              enabled: true,
              api_key: apiKey || '',
              secret_key: secretKey || '',
              passphrase: passphrase || '',
              testnet: testnet || false,
              hyperliquid_wallet_addr: hyperliquidWalletAddr || '',
              aster_user: asterUser || '',
              aster_signer: asterSigner || '',
              aster_private_key: asterPrivateKey || '',
              lighter_wallet_addr: lighterWalletAddr || '',
              lighter_private_key: lighterPrivateKey || '',
              lighter_api_key_private_key: lighterApiKeyPrivateKey || '',
              lighter_api_key_index: lighterApiKeyIndex || 0,
            },
          },
        }

        await toast.promise(api.updateExchangeConfigsEncrypted(request), {
          loading: 'Updating exchange config...',
          success: 'Exchange config updated',
          error: 'Failed to update exchange config',
        })
      } else {
        // 创建新账户
        const createRequest = {
          exchange_type: exchangeType,
          account_name: accountName,
          enabled: true,
          api_key: apiKey || '',
          secret_key: secretKey || '',
          passphrase: passphrase || '',
          testnet: testnet || false,
          hyperliquid_wallet_addr: hyperliquidWalletAddr || '',
          aster_user: asterUser || '',
          aster_signer: asterSigner || '',
          aster_private_key: asterPrivateKey || '',
          lighter_wallet_addr: lighterWalletAddr || '',
          lighter_private_key: lighterPrivateKey || '',
          lighter_api_key_private_key: lighterApiKeyPrivateKey || '',
          lighter_api_key_index: lighterApiKeyIndex || 0,
        }

        await toast.promise(api.createExchangeEncrypted(createRequest), {
          loading: 'Creating exchange account...',
          success: 'Exchange account created',
          error: 'Failed to create exchange account',
        })
      }

      // 重新获取用户配置以确保数据同步
      const refreshedExchanges = await api.getExchangeConfigs()
      setAllExchanges(refreshedExchanges)

      setShowExchangeModal(false)
      setEditingExchange(null)
    } catch (error) {
      console.error('Failed to save exchange config:', error)
      toast.error('Failed to save exchange config')
    }
  }

  const handleAddModel = () => {
    setEditingModel(null)
    setShowModelModal(true)
  }

  const handleAddExchange = () => {
    setEditingExchange(null)
    setShowExchangeModal(true)
  }

  return (
    <DeepVoidBackground className="py-8">
      <div className="w-full px-4 md:px-24 space-y-8">
        {/* Header - Terminal Style */}
        <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4 border-b border-white/10 pb-6">
          <div className="flex items-center gap-4">
            <div className="relative group">
              <div className="absolute -inset-1 bg-nofx-gold/20 rounded-xl blur opacity-0 group-hover:opacity-100 transition duration-500"></div>
              <div className="w-12 h-12 md:w-14 md:h-14 rounded-xl flex items-center justify-center bg-black border border-nofx-gold/30 text-nofx-gold relative z-10 shadow-[0_0_15px_rgba(240,185,11,0.1)]">
                <Bot className="w-6 h-6 md:w-7 md:h-7" />
              </div>
            </div>
            <div>
              <h1 className="text-2xl md:text-3xl font-bold font-mono tracking-tight text-white flex items-center gap-3 uppercase">
                AI_TRADERS
                <span className="text-xs font-mono font-normal px-2 py-0.5 rounded bg-nofx-gold/10 text-nofx-gold border border-nofx-gold/20 tracking-wider">
                  {traders?.length || 0} ACTIVE_NODES
                </span>
              </h1>
              <p className="text-xs font-mono text-zinc-500 uppercase tracking-widest mt-1 ml-1 flex items-center gap-2">
                <span className="w-2 h-2 rounded-full bg-green-500 animate-pulse"></span>
                SYSTEM_READY
              </p>
            </div>
          </div>

          <div className="flex flex-wrap gap-2 w-full md:w-auto">
            <button
              onClick={handleAddModel}
              className="flex-1 md:flex-none px-3 md:px-4 py-2 rounded text-xs font-mono uppercase tracking-wider transition-all border border-zinc-700 bg-black/20 text-zinc-400 hover:text-white hover:border-zinc-500 whitespace-nowrap backdrop-blur-sm"
            >
              <div className="flex items-center justify-center gap-1 md:gap-2">
                <Plus className="w-3 h-3" />
                <span className="hidden sm:inline">MODELS_CONFIG</span>
                <span className="sm:hidden">MODELS</span>
              </div>
            </button>

            <button
              onClick={handleAddExchange}
              className="flex-1 md:flex-none px-3 md:px-4 py-2 rounded text-xs font-mono uppercase tracking-wider transition-all border border-zinc-700 bg-black/20 text-zinc-400 hover:text-white hover:border-zinc-500 whitespace-nowrap backdrop-blur-sm"
            >
              <div className="flex items-center justify-center gap-1 md:gap-2">
                <Plus className="w-3 h-3" />
                <span className="hidden sm:inline">EXCHANGE_KEYS</span>
                <span className="sm:hidden">EXCHANGE</span>
              </div>
            </button>

            <button
              onClick={() => setShowCreateModal(true)}
              disabled={
                configuredModels.length === 0 ||
                configuredExchanges.length === 0
              }
              className="w-full sm:w-auto sm:flex-1 md:flex-none group relative px-4 md:px-6 py-2 rounded text-xs font-bold font-mono uppercase tracking-wider transition-all disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap overflow-hidden bg-nofx-gold text-black hover:bg-yellow-400 shadow-[0_0_20px_rgba(240,185,11,0.2)] hover:shadow-[0_0_30px_rgba(240,185,11,0.4)]"
            >
              <span className="relative z-10 flex items-center justify-center gap-2">
                <span className="hidden sm:inline">Create Trader</span>
                <span className="sm:hidden">CREATE</span>
              </span>
              <div className="absolute inset-0 bg-white/20 translate-y-full group-hover:translate-y-0 transition-transform duration-300"></div>
            </button>
          </div>
        </div>

        {/* Configuration Status Grid */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* AI Models Card */}
          <div className="nofx-glass rounded-lg border border-white/20 overflow-hidden bg-nofx-bg-lighter">
            <div className="px-4 py-3 border-b border-white/20 bg-black/20 flex items-center gap-2 backdrop-blur-sm">
              <Brain className="w-4 h-4 text-nofx-gold" />
              <h3 className="text-sm font-mono tracking-widest text-zinc-300 uppercase">
                AI_MODELS
              </h3>
            </div>

            <div className="p-4 space-y-3">
              {configuredModels.map((model) => {
                const inUse = isModelInUse(model.id)
                const usageInfo = getModelUsageInfo(model.id)
                return (
                  <div
                    key={model.id}
                    className={`group relative flex items-center justify-between p-3 rounded-md transition-all border border-transparent ${
                      inUse
                        ? 'opacity-80'
                        : 'hover:bg-white/5 hover:border-white/10 cursor-pointer'
                    } bg-black/20`}
                    onClick={() => handleModelClick(model.id)}
                  >
                    <div className="flex items-center gap-4">
                      <div className="relative">
                        <div className="absolute inset-0 bg-indigo-500/20 rounded-full blur-sm group-hover:bg-indigo-500/30 transition-all"></div>
                        <div className="w-10 h-10 rounded-full flex items-center justify-center bg-black border border-white/10 relative z-10">
                          {getModelIcon(model.provider || model.id, {
                            width: 20,
                            height: 20,
                          }) || (
                            <span className="text-xs font-bold text-indigo-400">
                              {getShortName(model.name)[0]}
                            </span>
                          )}
                        </div>
                      </div>

                      <div className="min-w-0">
                        <div className="font-mono text-sm text-zinc-200 group-hover:text-nofx-gold transition-colors">
                          {getShortName(model.name)}
                        </div>
                        <div className="text-[10px] text-zinc-500 font-mono flex items-center gap-2">
                          {model.customModelName ||
                            AI_PROVIDER_CONFIG[model.provider]?.defaultModel ||
                            ''}
                        </div>
                      </div>
                    </div>

                    <div className="text-right">
                      {usageInfo.totalCount > 0 ? (
                        <span
                          className={`text-[10px] font-mono px-2 py-1 rounded border ${
                            usageInfo.runningCount > 0
                              ? 'bg-green-500/10 border-green-500/30 text-green-400'
                              : 'bg-yellow-500/10 border-yellow-500/30 text-yellow-400'
                          }`}
                        >
                          {usageInfo.runningCount}/{usageInfo.totalCount} ACTIVE
                        </span>
                      ) : (
                        <span className="text-[10px] font-mono text-zinc-600 uppercase tracking-wider">
                          STANDBY
                        </span>
                      )}
                    </div>
                  </div>
                )
              })}

              {configuredModels.length === 0 && (
                <div className="text-center py-10 border border-dashed border-zinc-800 rounded-lg bg-black/20">
                  <Brain className="w-8 h-8 mx-auto mb-3 text-zinc-700" />
                  <div className="text-xs font-mono text-zinc-500 uppercase tracking-widest">
                    NO_MODELS_CONFIGURED
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Exchanges Card */}
          <div className="nofx-glass rounded-lg border border-white/20 overflow-hidden bg-nofx-bg-lighter">
            <div className="px-4 py-3 border-b border-white/20 bg-black/20 flex items-center gap-2 backdrop-blur-sm">
              <Landmark className="w-4 h-4 text-nofx-gold" />
              <h3 className="text-sm font-mono tracking-widest text-zinc-300 uppercase">
                EXCHANGES
              </h3>
            </div>

            <div className="p-4 space-y-3">
              {configuredExchanges.map((exchange) => {
                const inUse = isExchangeInUse(exchange.id)
                const usageInfo = getExchangeUsageInfo(exchange.id)
                return (
                  <div
                    key={exchange.id}
                    className={`group relative flex items-center justify-between p-3 rounded-md transition-all border border-transparent ${
                      inUse
                        ? 'opacity-80'
                        : 'hover:bg-white/5 hover:border-white/10 cursor-pointer'
                    } bg-black/20`}
                    onClick={() => handleExchangeClick(exchange.id)}
                  >
                    <div className="flex items-center gap-4 min-w-0">
                      <div className="relative">
                        <div className="absolute inset-0 bg-yellow-500/20 rounded-full blur-sm group-hover:bg-yellow-500/30 transition-all"></div>
                        <div className="w-10 h-10 rounded-full flex items-center justify-center bg-black border border-white/10 relative z-10">
                          {getExchangeIcon(
                            exchange.exchange_type || exchange.id,
                            { width: 20, height: 20 }
                          )}
                        </div>
                      </div>

                      <div className="min-w-0">
                        <div className="font-mono text-sm text-zinc-200 group-hover:text-nofx-gold transition-colors truncate">
                          {exchange.exchange_type?.toUpperCase() ||
                            getShortName(exchange.name)}
                          <span className="text-[10px] text-zinc-500 ml-2 border border-zinc-800 px-1 rounded">
                            {exchange.account_name || 'DEFAULT'}
                          </span>
                        </div>
                        <div className="text-[10px] text-zinc-500 font-mono flex items-center gap-2">
                          {exchange.type?.toUpperCase() || 'CEX'}
                        </div>
                      </div>
                    </div>

                    <div className="flex flex-col items-end gap-1">
                      {/* Wallet Address Display Logic */}
                      {(() => {
                        const walletAddr =
                          exchange.hyperliquidWalletAddr ||
                          exchange.asterUser ||
                          exchange.lighterWalletAddr
                        if (exchange.type !== 'dex' || !walletAddr) return null
                        const isVisible = visibleExchangeAddresses.has(
                          exchange.id
                        )
                        const isCopied = copiedId === `exchange-${exchange.id}`

                        return (
                          <div
                            className="flex items-center gap-1"
                            onClick={(e) => e.stopPropagation()}
                          >
                            <span className="text-[10px] font-mono text-zinc-400 bg-black/40 px-1.5 py-0.5 rounded border border-zinc-800">
                              {isVisible
                                ? walletAddr
                                : truncateAddress(walletAddr)}
                            </span>
                            <button
                              onClick={(e) => {
                                e.stopPropagation()
                                toggleExchangeAddressVisibility(exchange.id)
                              }}
                              className="text-zinc-600 hover:text-zinc-300"
                            >
                              {isVisible ? (
                                <EyeOff size={10} />
                              ) : (
                                <Eye size={10} />
                              )}
                            </button>
                            <button
                              onClick={(e) => {
                                e.stopPropagation()
                                handleCopyAddress(
                                  `exchange-${exchange.id}`,
                                  walletAddr
                                )
                              }}
                              className="text-zinc-600 hover:text-nofx-gold"
                            >
                              {isCopied ? (
                                <Check size={10} className="text-green-500" />
                              ) : (
                                <Copy size={10} />
                              )}
                            </button>
                          </div>
                        )
                      })()}

                      {usageInfo.totalCount > 0 ? (
                        <span
                          className={`text-[10px] font-mono px-2 py-1 rounded border ${
                            usageInfo.runningCount > 0
                              ? 'bg-green-500/10 border-green-500/30 text-green-400'
                              : 'bg-yellow-500/10 border-yellow-500/30 text-yellow-400'
                          }`}
                        >
                          {usageInfo.runningCount}/{usageInfo.totalCount} ACTIVE
                        </span>
                      ) : (
                        <span className="text-[10px] font-mono text-zinc-600 uppercase tracking-wider">
                          STANDBY
                        </span>
                      )}
                    </div>
                  </div>
                )
              })}
              {configuredExchanges.length === 0 && (
                <div className="text-center py-10 border border-dashed border-zinc-800 rounded-lg bg-black/20">
                  <Landmark className="w-8 h-8 mx-auto mb-3 text-zinc-700" />
                  <div className="text-xs font-mono text-zinc-500 uppercase tracking-widest">
                    NO_EXCHANGES_CONFIGURED
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Traders List */}
        <div className="nofx-glass rounded-lg border border-white/20 overflow-hidden bg-nofx-bg-lighter p-4 md:p-6">
          <div className="flex items-center justify-between mb-4 md:mb-5">
            <h2
              className="text-lg md:text-xl font-bold flex items-center gap-2"
              style={{ color: '#EAECEF' }}
            >
              <Users
                className="w-5 h-5 md:w-6 md:h-6"
                style={{ color: '#F0B90B' }}
              />
              CURRENT_TRADERS
            </h2>
          </div>

          {isTradersLoading ? (
            /* Loading Skeleton */
            <div className="space-y-3 md:space-y-4">
              {[1, 2, 3].map((i) => (
                <div
                  key={i}
                  className="flex flex-col md:flex-row md:items-center justify-between p-3 md:p-4 rounded gap-3 md:gap-4 animate-pulse"
                  style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
                >
                  <div className="flex items-center gap-3 md:gap-4">
                    <div className="w-10 h-10 md:w-12 md:h-12 rounded-full skeleton"></div>
                    <div className="min-w-0 space-y-2">
                      <div className="skeleton h-5 w-32"></div>
                      <div className="skeleton h-3 w-24"></div>
                    </div>
                  </div>
                  <div className="flex items-center gap-3 md:gap-4">
                    <div className="skeleton h-6 w-16"></div>
                    <div className="skeleton h-6 w-16"></div>
                    <div className="skeleton h-8 w-20"></div>
                  </div>
                </div>
              ))}
            </div>
          ) : traders && traders.length > 0 ? (
            <div className="space-y-3 md:space-y-4">
              {traders.map((trader) => (
                <div
                  key={trader.trader_id}
                  className="flex flex-col md:flex-row md:items-center justify-between p-3 md:p-4 rounded transition-all hover:translate-y-[-1px] gap-3 md:gap-4"
                  style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
                >
                  <div className="flex items-center gap-3 md:gap-4">
                    <div className="flex-shrink-0">
                      <PunkAvatar
                        seed={getTraderAvatar(
                          trader.trader_id,
                          trader.trader_name
                        )}
                        size={48}
                        className="rounded-lg hidden md:block"
                      />
                      <PunkAvatar
                        seed={getTraderAvatar(
                          trader.trader_id,
                          trader.trader_name
                        )}
                        size={40}
                        className="rounded-lg md:hidden"
                      />
                    </div>
                    <div className="min-w-0">
                      <div
                        className="font-bold text-base md:text-lg truncate"
                        style={{ color: '#EAECEF' }}
                      >
                        {trader.trader_name}
                      </div>
                      <div
                        className="text-xs md:text-sm truncate"
                        style={{
                          color: trader.ai_model.includes('deepseek')
                            ? '#60a5fa'
                            : '#c084fc',
                        }}
                      >
                        {getModelDisplayName(
                          trader.ai_model.split('_').pop() || trader.ai_model
                        )}{' '}
                        Model •{' '}
                        {getExchangeDisplayName(
                          trader.exchange_id,
                          allExchanges
                        )}
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center gap-3 md:gap-4 flex-wrap md:flex-nowrap">
                    {/* Wallet Address for Perp-DEX - placed before status for alignment */}
                    {(() => {
                      const exchange = allExchanges.find(
                        (e) => e.id === trader.exchange_id
                      )
                      const walletAddr = getWalletAddress(exchange)
                      const isPerpDex = isPerpDexExchange(
                        exchange?.exchange_type
                      )
                      if (!isPerpDex || !walletAddr) return null

                      const isVisible = visibleTraderAddresses.has(
                        trader.trader_id
                      )
                      const isCopied = copiedId === trader.trader_id

                      return (
                        <div
                          className="flex items-center gap-1 px-2 py-1 rounded"
                          style={{
                            background: 'rgba(240, 185, 11, 0.08)',
                            border: '1px solid rgba(240, 185, 11, 0.2)',
                          }}
                        >
                          <span
                            className="text-xs font-mono"
                            style={{ color: '#F0B90B' }}
                          >
                            {isVisible
                              ? walletAddr
                              : truncateAddress(walletAddr)}
                          </span>
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation()
                              toggleTraderAddressVisibility(trader.trader_id)
                            }}
                            className="p-0.5 rounded hover:bg-gray-700 transition-colors"
                            title={isVisible ? 'Hide' : 'Show'}
                          >
                            {isVisible ? (
                              <EyeOff
                                className="w-3 h-3"
                                style={{ color: '#848E9C' }}
                              />
                            ) : (
                              <Eye
                                className="w-3 h-3"
                                style={{ color: '#848E9C' }}
                              />
                            )}
                          </button>
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation()
                              handleCopyAddress(trader.trader_id, walletAddr)
                            }}
                            className="p-0.5 rounded hover:bg-gray-700 transition-colors"
                            title="Copy"
                          >
                            {isCopied ? (
                              <Check
                                className="w-3 h-3"
                                style={{ color: '#0ECB81' }}
                              />
                            ) : (
                              <Copy
                                className="w-3 h-3"
                                style={{ color: '#848E9C' }}
                              />
                            )}
                          </button>
                        </div>
                      )
                    })()}
                    {/* Status */}
                    <div className="text-center">
                      <div
                        className={`px-2 md:px-3 py-1 rounded text-xs font-bold ${
                          trader.is_running
                            ? 'bg-green-100 text-green-800'
                            : 'bg-red-100 text-red-800'
                        }`}
                        style={
                          trader.is_running
                            ? {
                                background: 'rgba(14, 203, 129, 0.1)',
                                color: '#0ECB81',
                              }
                            : {
                                background: 'rgba(246, 70, 93, 0.1)',
                                color: '#F6465D',
                              }
                        }
                      >
                        {trader.is_running ? 'RUNNING' : 'STOPPED'}
                      </div>
                    </div>

                    {/* Actions: 禁止换行，超出横向滚动 */}
                    <div className="flex gap-1.5 md:gap-2 flex-nowrap overflow-x-auto items-center">
                      <button
                        onClick={() => {
                          if (onTraderSelect) {
                            onTraderSelect(trader.trader_id)
                          } else {
                            // 使用 slug 格式: name-id前4位
                            const slug = `${trader.trader_name}-${trader.trader_id.slice(0, 4)}`
                            navigate(
                              `/dashboard?trader=${encodeURIComponent(slug)}`
                            )
                          }
                        }}
                        className="px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 flex items-center gap-1 whitespace-nowrap"
                        style={{
                          background: 'rgba(99, 102, 241, 0.1)',
                          color: '#6366F1',
                        }}
                      >
                        <BarChart3 className="w-3 h-3 md:w-4 md:h-4" />
                        VIEW
                      </button>

                      <button
                        onClick={() => handleEditTrader(trader.trader_id)}
                        disabled={trader.is_running}
                        className="px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap flex items-center gap-1"
                        style={{
                          background: trader.is_running
                            ? 'rgba(132, 142, 156, 0.1)'
                            : 'rgba(255, 193, 7, 0.1)',
                          color: trader.is_running ? '#848E9C' : '#FFC107',
                        }}
                      >
                        <Pencil className="w-3 h-3 md:w-4 md:h-4" />
                        EDIT
                      </button>

                      <button
                        onClick={() =>
                          handleToggleTrader(
                            trader.trader_id,
                            trader.is_running || false
                          )
                        }
                        disabled={togglingTraders.has(trader.trader_id)}
                        className={`px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all whitespace-nowrap flex items-center gap-1 ${
                          togglingTraders.has(trader.trader_id)
                            ? 'opacity-70 cursor-wait'
                            : 'hover:scale-105'
                        }`}
                        style={
                          trader.is_running
                            ? {
                                background: 'rgba(246, 70, 93, 0.1)',
                                color: '#F6465D',
                              }
                            : {
                                background: 'rgba(14, 203, 129, 0.1)',
                                color: '#0ECB81',
                              }
                        }
                      >
                        {togglingTraders.has(trader.trader_id) ? (
                          <svg
                            className="w-3 h-3 md:w-4 md:h-4 animate-spin"
                            fill="none"
                            viewBox="0 0 24 24"
                          >
                            <circle
                              className="opacity-25"
                              cx="12"
                              cy="12"
                              r="10"
                              stroke="currentColor"
                              strokeWidth="4"
                            />
                            <path
                              className="opacity-75"
                              fill="currentColor"
                              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                            />
                          </svg>
                        ) : null}
                        {trader.is_running ? 'STOP' : 'START'}
                      </button>

                      <button
                        onClick={() =>
                          handleToggleCompetition(
                            trader.trader_id,
                            trader.show_in_competition ?? true
                          )
                        }
                        className="px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 whitespace-nowrap flex items-center gap-1"
                        style={
                          trader.show_in_competition !== false
                            ? {
                                background: 'rgba(14, 203, 129, 0.1)',
                                color: '#0ECB81',
                              }
                            : {
                                background: 'rgba(132, 142, 156, 0.1)',
                                color: '#848E9C',
                              }
                        }
                        title={
                          trader.show_in_competition !== false
                            ? 'SHOW_IN_COMPETITION'
                            : 'HIDE_IN_COMPETITION'
                        }
                      >
                        {trader.show_in_competition !== false ? (
                          <Eye className="w-3 h-3 md:w-4 md:h-4" />
                        ) : (
                          <EyeOff className="w-3 h-3 md:w-4 md:h-4" />
                        )}
                      </button>

                      <button
                        onClick={() => handleDeleteTrader(trader.trader_id)}
                        className="px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105"
                        style={{
                          background: 'rgba(246, 70, 93, 0.1)',
                          color: '#F6465D',
                        }}
                      >
                        <Trash2 className="w-3 h-3 md:w-4 md:h-4" />
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div
              className="text-center py-12 md:py-16"
              style={{ color: '#848E9C' }}
            >
              <Bot className="w-16 h-16 md:w-24 md:h-24 mx-auto mb-3 md:mb-4 opacity-50" />
              <div className="text-base md:text-lg font-semibold mb-2">
                No Traders
              </div>
              <div className="text-xs md:text-sm mb-3 md:mb-4">
                Create First Trader
              </div>
              {(configuredModels.length === 0 ||
                configuredExchanges.length === 0) && (
                <div className="text-xs md:text-sm text-yellow-500">
                  {configuredModels.length === 0 &&
                  configuredExchanges.length === 0
                    ? 'Configure Models and Exchanges First'
                    : configuredModels.length === 0
                      ? 'Configure Models First'
                      : 'Configure Exchanges First'}
                </div>
              )}
            </div>
          )}
        </div>

        {/* Create Trader Modal */}
        {showCreateModal && (
          <TraderConfigModal
            isOpen={showCreateModal}
            isEditMode={false}
            availableModels={enabledModels}
            availableExchanges={enabledExchanges}
            onSave={handleCreateTrader}
            onClose={() => setShowCreateModal(false)}
          />
        )}

        {/* Edit Trader Modal */}
        {showEditModal && editingTrader && (
          <TraderConfigModal
            isOpen={showEditModal}
            isEditMode={true}
            traderData={editingTrader}
            availableModels={enabledModels}
            availableExchanges={enabledExchanges}
            onSave={handleSaveEditTrader}
            onClose={() => {
              setShowEditModal(false)
              setEditingTrader(null)
            }}
          />
        )}

        {/* Model Configuration Modal */}
        {showModelModal && (
          <ModelConfigModal
            allModels={supportedModels}
            configuredModels={allModels}
            editingModelId={editingModel}
            onSave={handleSaveModelConfig}
            onDelete={handleDeleteModelConfig}
            onClose={() => {
              setShowModelModal(false)
              setEditingModel(null)
            }}
            language={language}
          />
        )}

        {/* Exchange Configuration Modal */}
        {showExchangeModal && (
          <ExchangeConfigModal
            allExchanges={allExchanges}
            editingExchangeId={editingExchange}
            onSave={handleSaveExchangeConfig}
            onDelete={handleDeleteExchangeConfig}
            onClose={() => {
              setShowExchangeModal(false)
              setEditingExchange(null)
            }}
            language={language}
          />
        )}
      </div>
    </DeepVoidBackground>
  )
}

// Model Configuration Modal Component
function ModelConfigModal({
  allModels,
  configuredModels,
  editingModelId,
  onSave,
  onDelete,
  onClose,
  language,
}: {
  allModels: AIModel[]
  configuredModels: AIModel[]
  editingModelId: string | null
  onSave: (
    modelId: string,
    apiKey: string,
    baseUrl?: string,
    modelName?: string
  ) => void
  onDelete: (modelId: string) => void
  onClose: () => void
  language: Language
}) {
  const [selectedModelId, setSelectedModelId] = useState(editingModelId || '')
  const [apiKey, setApiKey] = useState('')
  const [baseUrl, setBaseUrl] = useState('')
  const [modelName, setModelName] = useState('')

  // 获取当前编辑的模型信息 - 编辑时从已配置的模型中查找，新建时从所有支持的模型中查找
  const selectedModel = editingModelId
    ? configuredModels?.find((m) => m.id === selectedModelId)
    : allModels?.find((m) => m.id === selectedModelId)

  // 如果是编辑现有模型，初始化API Key、Base URL和Model Name
  useEffect(() => {
    if (editingModelId && selectedModel) {
      setApiKey(selectedModel.apiKey || '')
      setBaseUrl(selectedModel.customApiUrl || '')
      setModelName(selectedModel.customModelName || '')
    }
  }, [editingModelId, selectedModel])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedModelId || !apiKey.trim()) return

    onSave(
      selectedModelId,
      apiKey.trim(),
      baseUrl.trim() || undefined,
      modelName.trim() || undefined
    )
  }

  const availableModels = allModels || []

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4 overflow-y-auto">
      <div
        className="bg-gray-800 rounded-lg w-full max-w-lg relative my-8"
        style={{
          background: '#1E2329',
          maxHeight: 'calc(100vh - 6rem)',
        }}
      >
        <div
          className="flex items-center justify-between p-3 md:p-6 pb-4 sticky top-0 z-10 rounded-t-lg"
          style={{ background: '#1E2329' }}
        >
          <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
            {editingModelId ? 'Edit AI Model' : 'Add AI Model'}
          </h3>

          <div className="flex items-center gap-2">
            <button
              onClick={onClose}
              className="w-8 h-8 rounded-lg text-[#848E9C] hover:text-[#EAECEF] hover:bg-[#2B3139] transition-colors flex items-center justify-center"
            >
              <IconX className="w-4 h-4" />
            </button>
          </div>
        </div>

        <form onSubmit={handleSubmit} className="px-3 md:px-6 pb-6">
          <div
            className="space-y-4 overflow-y-auto"
            style={{ maxHeight: 'calc(100vh - 16rem)' }}
          >
            {!editingModelId && (
              <div>
                <label
                  className="block text-sm font-semibold mb-2"
                  style={{ color: '#EAECEF' }}
                >
                  Select Model
                </label>
                <CustomSelect
                  value={selectedModelId}
                  onChange={(val) => setSelectedModelId(String(val))}
                  placeholder="Select Model"
                  options={[
                    { value: '', label: 'Select Model' },
                    ...availableModels.map((model) => ({
                      value: model.id,
                      label: `${getShortName(model.name)} (${model.provider})`,
                    })),
                  ]}
                  className="w-full"
                />
              </div>
            )}

            {selectedModel && (
              <div
                className="p-4 rounded"
                style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
              >
                <div className="flex items-center gap-3 mb-3">
                  <div className="w-8 h-8 flex items-center justify-center">
                    {getModelIcon(selectedModel.provider || selectedModel.id, {
                      width: 32,
                      height: 32,
                    }) || (
                      <div
                        className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold"
                        style={{
                          background:
                            selectedModel.id === 'deepseek'
                              ? '#60a5fa'
                              : '#c084fc',
                          color: '#fff',
                        }}
                      >
                        {selectedModel.name[0]}
                      </div>
                    )}
                  </div>
                  <div className="flex-1">
                    <div className="font-semibold" style={{ color: '#EAECEF' }}>
                      {getShortName(selectedModel.name)}
                    </div>
                    <div className="text-xs" style={{ color: '#848E9C' }}>
                      {selectedModel.provider} • {selectedModel.id}
                    </div>
                  </div>
                </div>
                {/* Default model info and API link */}
                {AI_PROVIDER_CONFIG[selectedModel.provider] && (
                  <div
                    className="mt-3 pt-3"
                    style={{ borderTop: '1px solid #2B3139' }}
                  >
                    <div className="text-xs mb-2" style={{ color: '#848E9C' }}>
                      Default Model:{' '}
                      <span style={{ color: '#F0B90B' }}>
                        {
                          AI_PROVIDER_CONFIG[selectedModel.provider]
                            .defaultModel
                        }
                      </span>
                    </div>
                    <a
                      href={AI_PROVIDER_CONFIG[selectedModel.provider].apiUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1.5 text-xs hover:underline"
                      style={{ color: '#F0B90B' }}
                    >
                      <ExternalLink className="w-3 h-3" />
                      Apply API Key →{' '}
                      {AI_PROVIDER_CONFIG[selectedModel.provider].apiName}
                    </a>
                    {selectedModel.provider === 'kimi' && (
                      <div
                        className="mt-2 text-xs p-2 rounded"
                        style={{
                          background: 'rgba(246, 70, 93, 0.1)',
                          color: '#F6465D',
                        }}
                      >
                        ⚠️ {t('kimiApiNote', language)}
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}

            {selectedModel && (
              <>
                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: '#EAECEF' }}
                  >
                    API Key
                  </label>
                  <input
                    type="password"
                    value={apiKey}
                    onChange={(e) => setApiKey(e.target.value)}
                    placeholder="Enter API Key"
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: '#0B0E11',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                    required
                  />
                </div>

                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: '#EAECEF' }}
                  >
                    Custom Base URL
                  </label>
                  <input
                    type="url"
                    value={baseUrl}
                    onChange={(e) => setBaseUrl(e.target.value)}
                    placeholder="Custom Base URL"
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: '#0B0E11',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                  />
                  <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                    Leave blank for default
                  </div>
                </div>

                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: '#EAECEF' }}
                  >
                    Custom Model Name
                  </label>
                  <input
                    type="text"
                    value={modelName}
                    onChange={(e) => setModelName(e.target.value)}
                    placeholder="Custom Model Name"
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: '#0B0E11',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                  />
                  <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                    Leave blank for default model
                  </div>
                </div>
              </>
            )}
          </div>

          <div
            className="flex gap-3 mt-6 pt-4 sticky bottom-0"
            style={{ background: '#1E2329' }}
          >
            {editingModelId && (
              <button
                type="button"
                onClick={() => onDelete(editingModelId)}
                className="flex-1 px-4 py-3 rounded text-sm font-semibold bg-red-400/50 hover:bg-red-600/50"
                title="Delete"
              >
                Delete
              </button>
            )}
            <button
              type="submit"
              disabled={!selectedModel || !apiKey.trim()}
              className="flex-1 px-4 py-3 rounded text-sm font-semibold disabled:opacity-50"
              style={{ background: '#F0B90B', color: '#000' }}
            >
              Save
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
