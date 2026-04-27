import { motion, type Variants } from 'framer-motion';
import { useSettings } from '@/lib/settings';
import { Toggle } from '@/components/controls/Toggle';

const pageVariants: Variants = {
  hidden: { opacity: 0 },
  show: {
    opacity: 1,
    transition: { delayChildren: 0.05, staggerChildren: 0.04 },
  },
};

const sectionVariants: Variants = {
  hidden: { opacity: 0, y: 8 },
  show: { opacity: 1, y: 0, transition: { duration: 0.24, ease: [0.16, 1, 0.3, 1] } },
};

export function Settings() {
  const [s, update] = useSettings();

  return (
    <motion.section
      className="flex flex-col gap-3"
      initial="hidden"
      animate="show"
      variants={pageVariants}
    >
      <h1 className="text-[22px] font-semibold tracking-tight">Settings</h1>

      <motion.div variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <div className="flex items-baseline justify-between pb-3 mb-1 border-b border-white/[0.08]">
          <span className="text-[14px] font-semibold tracking-wide text-white/[0.92]">General</span>
        </div>

        <div className="flex items-center justify-between py-3">
          <div className="flex flex-col gap-0.5 max-w-[60%]">
            <span className="text-[13px] text-white/[0.92]">Launch on system startup</span>
            <span className="text-[11.5px] leading-snug text-white/[0.45]">
              Start ITG Ray automatically when you log in.
            </span>
          </div>
          <Toggle value={s.launchOnStartup} onChange={(v) => update({ launchOnStartup: v })} />
        </div>

        <div className="flex items-center justify-between py-3 border-t border-white/[0.06]">
          <div className="flex flex-col gap-0.5 max-w-[60%]">
            <span className="text-[13px] text-white/[0.92]">Start minimized to tray</span>
            <span className="text-[11.5px] leading-snug text-white/[0.45]">
              Open in background, no main window on launch.
            </span>
          </div>
          <Toggle value={s.startMinimized} onChange={(v) => update({ startMinimized: v })} />
        </div>
      </motion.div>
    </motion.section>
  );
}
