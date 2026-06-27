import { Language } from '../../i18n/translations'

interface FooterSectionProps {
  language: Language
}

export default function FooterSection({ language }: FooterSectionProps) {
  // Links variable unused - kept for future reference
  // const links = {
  //   social: [
  //     { name: 'GitHub', href: OFFICIAL_LINKS.github, icon: Github },
  //     {
  //       name: 'X (Twitter)',
  //       href: OFFICIAL_LINKS.twitter,
  //       icon: () => (
  //         <svg viewBox="0 0 24 24" className="w-4 h-4" fill="currentColor">
  //           <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
  //         </svg>
  //       ),
  //     },
  //     { name: 'Telegram', href: OFFICIAL_LINKS.telegram, icon: Send },
  //   ],
  //   resources: [
  //     {
  //       name: language === 'zh' ? '文档' : 'Documentation',
  //       href: 'https://github.com/NoFxAiOS/nofx/blob/main/README.md',
  //     },
  //     { name: 'Issues', href: 'https://github.com/NoFxAiOS/nofx/issues' },
  //     { name: 'Pull Requests', href: 'https://github.com/NoFxAiOS/nofx/pulls' },
  //   ],
  //   supporters: [
  //     { name: 'Binance', href: 'https://www.binance.com/join?ref=NOFXENG' },
  //     { name: 'Bybit', href: 'https://partner.bybit.com/b/83856' },
  //     { name: 'OKX', href: 'https://www.okx.com/join/1865360' },
  //     {
  //       name: 'Bitget',
  //       href: 'https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172',
  //     },
  //     {
  //       name: 'Hyperliquid',
  //       href: 'https://app.hyperliquid.xyz/join/AITRADING',
  //     },
  //     {
  //       name: 'Aster DEX',
  //       href: 'https://www.asterdex.com/en/referral/fdfc0e',
  //     },
  //     { name: 'Lighter', href: 'https://app.lighter.xyz/?referral=68151432' },
  //   ],
  // }

  return (
    <footer
      style={{
        background: '#0B0E11',
        borderTop: '1px solid rgba(255, 255, 255, 0.06)',
      }}
    >
      <div className="max-w-6xl mx-auto px-4 py-8 md:py-12">
        <div
          className="pt-6 text-center text-xs"
          style={{
            color: '#5E6673',
            borderTop: '1px solid rgba(255, 255, 255, 0.06)',
          }}
        >
          <p className="mb-2">HFX - AI Trading System</p>
          <p style={{ color: '#3C4249' }}>
            Trading involves risk. Use at your own discretion. - Version 1.0.0
          </p>
        </div>
      </div>
    </footer>
  )
}
