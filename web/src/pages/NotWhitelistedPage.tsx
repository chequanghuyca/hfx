import { motion } from 'framer-motion'
import { ArrowLeft, ShieldAlert } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { ROUTES } from '../routes'

export function NotWhitelistedPage() {
  const navigate = useNavigate()

  return (
    <div className="min-h-screen bg-[#05070a] flex items-center justify-center p-4 relative overflow-hidden">
      {/* Background Cinematic FX */}
      <div className="absolute inset-0 bg-vignette opacity-60" />
      <div
        className="absolute inset-0 opacity-20"
        style={{
          backgroundImage:
            'radial-gradient(circle at 50% 50%, #f0b90b 0%, transparent 70%)',
        }}
      />

      <motion.div
        initial={{ opacity: 0, scale: 0.9 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ duration: 0.5 }}
        className="relative z-10 max-w-md w-full"
      >
        <div className="bg-[#0b0e11]/80 backdrop-blur-xl border border-white/10 rounded-3xl p-8 shadow-2xl text-center">
          <div className="mb-6 flex justify-center">
            <div className="w-14 h-14 sm:w-20 sm:h-20 bg-red-500/10 rounded-2xl flex items-center justify-center border border-red-500/20">
              <ShieldAlert className="w-8 h-8 sm:w-12 sm:h-12 text-red-500" />
            </div>
          </div>

          <h1 className="text-xl sm:text-2xl font-bold text-white mb-4">
            Access Denied
          </h1>

          <div className="space-y-4 mb-8">
            <p className="text-[11px] sm:text-[14px] text-gray-400">
              Sorry, your email is not on the system's whitelist.
            </p>
            <div className="p-4 bg-white/5 rounded-xl border border-white/5">
              <p className="text-[11px] sm:text-[14px] text-nofx-gold font-medium flex items-center justify-center gap-2">
                Please contact the administrator for access.
              </p>
            </div>
          </div>

          <div className="space-y-3">
            <button
              onClick={() => navigate(ROUTES.HOME)}
              className="w-full py-3 bg-white/5 hover:bg-white/10 text-white font-medium rounded-xl border border-white/10 transition-all flex items-center justify-center gap-2 text-[11px] sm:text-[14px]"
            >
              <ArrowLeft className="w-4 h-4" />
              Back to Home
            </button>
          </div>
        </div>

        {/* CRT Overlay Effect */}
        <div className="absolute inset-0 crt-overlay opacity-[0.03] pointer-events-none rounded-3xl" />
      </motion.div>
    </div>
  )
}
