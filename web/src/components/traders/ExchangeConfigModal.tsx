import { X as IconX } from 'lucide-react'
import React, { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { t, type Language } from '../../i18n/translations'
import { api } from '../../lib/api'
import type { Exchange } from '../../types'
import { getExchangeIcon } from '../ExchangeIcons'
import {
  WebCryptoEnvironmentCheck,
  type WebCryptoCheckStatus,
} from '../WebCryptoEnvironmentCheck'
import { CustomSelect } from '../ui/CustomSelect'
import { getShortName } from './utils'

// Supported exchange templates for creating new accounts
const SUPPORTED_EXCHANGE_TEMPLATES = [
  { exchange_type: 'binance', name: 'Binance Futures', type: 'cex' as const },
]

interface ExchangeConfigModalProps {
  allExchanges: Exchange[]
  editingExchangeId: string | null
  onSave: (
    exchangeId: string | null,
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
  ) => Promise<void>
  onDelete: (exchangeId: string) => void
  onClose: () => void
  language: Language
}

export function ExchangeConfigModal({
  allExchanges,
  editingExchangeId,
  onSave,
  onDelete,
  onClose,
  language,
}: ExchangeConfigModalProps) {
  // Selected exchange type for creating new accounts
  const [selectedExchangeType, setSelectedExchangeType] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [secretKey, setSecretKey] = useState('')
  const [testnet, setTestnet] = useState(false)
  const [serverIP, setServerIP] = useState<{
    public_ip: string
    message: string
  } | null>(null)
  const [loadingIP, setLoadingIP] = useState(false)
  const [copiedIP, setCopiedIP] = useState(false)
  const [webCryptoStatus, setWebCryptoStatus] =
    useState<WebCryptoCheckStatus>('idle')


  // 保存中状态
  const [isSaving, setIsSaving] = useState(false)

  // 账户名称
  const [accountName, setAccountName] = useState('')

  // 获取当前编辑的交易所信息或模板
  // For editing: find the existing account by id (UUID)
  // For creating: use the selected exchange template
  const selectedExchange = editingExchangeId
    ? allExchanges?.find((e) => e.id === editingExchangeId)
    : null

  // Get the exchange template for displaying UI fields
  const selectedTemplate = editingExchangeId
    ? SUPPORTED_EXCHANGE_TEMPLATES.find(
        (t) => t.exchange_type === selectedExchange?.exchange_type
      )
    : SUPPORTED_EXCHANGE_TEMPLATES.find(
        (t) => t.exchange_type === selectedExchangeType
      )

  // Get the current exchange type (from existing account or selected template)
  const currentExchangeType = editingExchangeId
    ? selectedExchange?.exchange_type
    : selectedExchangeType

  // 如果是编辑现有交易所，初始化表单数据
  useEffect(() => {
    if (editingExchangeId && selectedExchange) {
      setAccountName(selectedExchange.account_name || '')
      setApiKey(selectedExchange.apiKey || '')
      setSecretKey(selectedExchange.secretKey || '')
      setTestnet(selectedExchange.testnet || false)
    }
  }, [editingExchangeId, selectedExchange])

  // 加载服务器IP（当选择binance时）
  useEffect(() => {
    if (currentExchangeType === 'binance' && !serverIP) {
      setLoadingIP(true)
      api
        .getServerIP()
        .then((data) => {
          setServerIP(data)
        })
        .catch((err) => {
          console.error('Failed to load server IP:', err)
        })
        .finally(() => {
          setLoadingIP(false)
        })
    }
  }, [currentExchangeType])

  const handleCopyIP = async (ip: string) => {
    try {
      // 优先使用现代 Clipboard API
      if (navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(ip)
        setCopiedIP(true)
        setTimeout(() => setCopiedIP(false), 2000)
        toast.success(t('ipCopied', language))
      } else {
        // 降级方案: 使用传统的 execCommand 方法
        const textArea = document.createElement('textarea')
        textArea.value = ip
        textArea.style.position = 'fixed'
        textArea.style.left = '-999999px'
        textArea.style.top = '-999999px'
        document.body.appendChild(textArea)
        textArea.focus()
        textArea.select()

        try {
          const successful = document.execCommand('copy')
          if (successful) {
            setCopiedIP(true)
            setTimeout(() => setCopiedIP(false), 2000)
            toast.success(t('ipCopied', language))
          } else {
            throw new Error('复制命令执行失败')
          }
        } finally {
          document.body.removeChild(textArea)
        }
      }
    } catch (err) {
      console.error('复制失败:', err)
      // 显示错误提示
      toast.error(
        t('copyIPFailed', language) || `复制失败: ${ip}\n请手动复制此IP地址`
      )
    }
  }


  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (isSaving) return

    // For creating, we need the exchange type
    if (!editingExchangeId && !selectedExchangeType) return

    // Validate account name
    const trimmedAccountName = accountName.trim()
    if (!trimmedAccountName) {
      toast.error(
        language === 'zh' ? '请输入账户名称' : 'Please enter account name'
      )
      return
    }

    const exchangeId = editingExchangeId || null
    const exchangeType = currentExchangeType || ''

    setIsSaving(true)
    try {
      // Only Binance is supported now
      if (currentExchangeType === 'binance' || true) {
        if (!apiKey.trim() || !secretKey.trim()) {
          toast.error(language === 'zh' ? '请输入API Key和Secret Key' : 'Please enter API Key and Secret Key')
          setIsSaving(false)
          return
        }
        await onSave(
          exchangeId,
          exchangeType,
          trimmedAccountName,
          apiKey.trim(),
          secretKey.trim(),
          undefined, // passphrase
          testnet
        )
      }
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4 overflow-y-visible">
      <div
        className="bg-gray-800 w-full max-w-3xl relative rounded-lg"
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
            {editingExchangeId ? 'Edit Exchange' : 'Add Exchange'}
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
            {!editingExchangeId && (
              <div className="space-y-3">
                <div className="space-y-2">
                  <div
                    className="text-xs font-semibold uppercase tracking-wide"
                    style={{ color: '#F0B90B' }}
                  >
                    {/* {t('environmentSteps.checkTitle', language)} */}
                    Environment Check
                  </div>
                  <WebCryptoEnvironmentCheck
                    language={language}
                    variant="card"
                    onStatusChange={setWebCryptoStatus}
                  />
                </div>
                <div className="space-y-2">
                  <div
                    className="text-xs font-semibold uppercase tracking-wide"
                    style={{ color: '#F0B90B' }}
                  >
                    {t('environmentSteps.selectTitle', language)}
                  </div>
                  <CustomSelect
                    value={selectedExchangeType}
                    onChange={(val) => setSelectedExchangeType(String(val))}
                    placeholder={t('pleaseSelectExchange', language)}
                    options={[
                      { value: '', label: t('pleaseSelectExchange', language) },
                      ...SUPPORTED_EXCHANGE_TEMPLATES.map((template) => ({
                        value: template.exchange_type,
                        label: `${getShortName(template.name)} (${template.type.toUpperCase()})`,
                      })),
                    ]}
                    className="w-full"
                    disabled={
                      webCryptoStatus !== 'secure' &&
                      webCryptoStatus !== 'disabled'
                    }
                  />
                </div>
              </div>
            )}

            {selectedTemplate && (
              <div
                className="p-4 rounded"
                style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
              >
                <div className="flex items-center gap-3 mb-3">
                  <div className="w-8 h-8 flex items-center justify-center">
                    {getExchangeIcon(selectedTemplate.exchange_type, {
                      width: 32,
                      height: 32,
                    })}
                  </div>
                  <div>
                    <div className="font-semibold" style={{ color: '#EAECEF' }}>
                      {getShortName(selectedTemplate.name)}
                      {editingExchangeId && selectedExchange?.account_name && (
                        <span
                          className="text-sm font-normal ml-2"
                          style={{ color: '#848E9C' }}
                        >
                          - {selectedExchange.account_name}
                        </span>
                      )}
                    </div>
                    <div className="text-xs" style={{ color: '#848E9C' }}>
                      {selectedTemplate.type.toUpperCase()} •{' '}
                      {selectedTemplate.exchange_type}
                    </div>
                  </div>
                </div>

                {/* 账户名称输入 */}
                <div className="mt-3">
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: '#EAECEF' }}
                  >
                    Account Name *
                  </label>
                  <input
                    type="text"
                    value={accountName}
                    onChange={(e) => setAccountName(e.target.value)}
                    placeholder={
                      language === 'zh'
                        ? '例如：主账户、套利账户'
                        : 'e.g., Main Account, Arbitrage Account'
                    }
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: '#1E2329',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                    required
                  />
                </div>
              </div>
            )}

            {selectedTemplate && (
              <>
                {/* Binance/Bybit/OKX/Bitget 的输入字段 */}
                {(currentExchangeType === 'binance' ||
                  currentExchangeType === 'bybit' ||
                  currentExchangeType === 'okx' ||
                  currentExchangeType === 'bitget') && (
                  <>
                    <div>
                      <label
                        className="block text-sm font-semibold mb-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('apiKey', language)}
                      </label>
                      <input
                        type="password"
                        value={apiKey}
                        onChange={(e) => setApiKey(e.target.value)}
                        placeholder={t('enterAPIKey', language)}
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
                        {t('secretKey', language)}
                      </label>
                      <input
                        type="password"
                        value={secretKey}
                        onChange={(e) => setSecretKey(e.target.value)}
                        placeholder={t('enterSecretKey', language)}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                        required
                      />
                    </div>


                    {/* Binance 白名单IP提示 */}
                    {currentExchangeType === 'binance' && (
                      <div
                        className="p-4 rounded"
                        style={{
                          background: 'rgba(240, 185, 11, 0.1)',
                          border: '1px solid rgba(240, 185, 11, 0.2)',
                        }}
                      >
                        <div
                          className="text-sm font-semibold mb-2"
                          style={{ color: '#F0B90B' }}
                        >
                          {t('whitelistIP', language)}
                        </div>
                        <div
                          className="text-xs mb-3"
                          style={{ color: '#848E9C' }}
                        >
                          {t('whitelistIPDesc', language)}
                        </div>

                        {loadingIP ? (
                          <div className="text-xs" style={{ color: '#848E9C' }}>
                            {t('loadingServerIP', language)}
                          </div>
                        ) : serverIP && serverIP.public_ip ? (
                          <div
                            className="flex items-center gap-2 p-2 rounded"
                            style={{ background: '#0B0E11' }}
                          >
                            <code
                              className="flex-1 text-sm font-mono"
                              style={{ color: '#F0B90B' }}
                            >
                              {serverIP.public_ip}
                            </code>
                            <button
                              type="button"
                              onClick={() => handleCopyIP(serverIP.public_ip)}
                              className="px-3 py-1 rounded text-xs font-semibold transition-all hover:scale-105"
                              style={{
                                background: 'rgba(240, 185, 11, 0.2)',
                                color: '#F0B90B',
                              }}
                            >
                              {copiedIP
                                ? t('ipCopied', language)
                                : t('copyIP', language)}
                            </button>
                          </div>
                        ) : null}
                      </div>
                    )}
                  </>
                )}

              </>
            )}
          </div>

          <div
            className="flex gap-3 mt-6 pt-4 sticky bottom-0"
            style={{ background: '#1E2329' }}
          >
            {editingExchangeId && (
              <button
                type="button"
                onClick={() => onDelete(editingExchangeId)}
                className="flex-1 px-4 py-2 rounded text-sm font-semibold bg-red-400/50 hover:bg-red-600/50"
                title="Delete"
              >
                Delete
              </button>
            )}
            <button
              type="submit"
              disabled={
                isSaving ||
                !selectedTemplate ||
                !accountName.trim() ||
                (currentExchangeType === 'binance' &&
                  (!apiKey.trim() || !secretKey.trim()))
              }
              className="flex-1 px-4 py-3 rounded text-sm font-semibold disabled:opacity-50"
              style={{ background: '#F0B90B', color: '#000' }}
            >
              {isSaving
                ? t('saving', language) || '保存中...'
                : t('saveConfig', language)}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
