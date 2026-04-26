import { useState } from "react";
import { StepWelcome } from "./StepWelcome";
import { StepFirstSub } from "./StepFirstSub";
import {
  Complete as wailsComplete,
  Skip as wailsSkip,
} from "../../../wailsjs/go/bindings/OnboardingService";
import { useStore } from "@/store";

// Wails generates TS signatures with a leading context.Context arg the
// runtime injects transparently. Cast to clean shapes for readability.
const Complete = wailsComplete as unknown as () => Promise<void>;
const Skip = wailsSkip as unknown as () => Promise<void>;

// Wizard is the first-run modal. AppShell mounts it conditionally on
// `!onboarded`; the two-step flow (Welcome → first subscription) writes
// the .onboarded marker on either Complete or Skip and flips the store
// flag so the modal disappears without a reload.
export function Wizard() {
  const [step, setStep] = useState<0 | 1>(0);

  const finish = async (skipped: boolean) => {
    try {
      if (skipped) await Skip();
      else await Complete();
    } finally {
      // Optimistically flip the flag even if the marker write failed —
      // the wizard is dismissable, and persisting failure should not
      // trap the user behind a modal they cannot close.
      useStore.setState({ onboarded: true });
    }
  };

  return (
    <div className="fixed inset-0 bg-black/60 grid place-items-center z-50">
      <div className="bg-surface-base border border-white/10 rounded-2xl p-6 w-[520px] max-w-[92vw]">
        {step === 0 ? (
          <StepWelcome onNext={() => setStep(1)} />
        ) : (
          <StepFirstSub onDone={() => finish(false)} onSkip={() => finish(true)} />
        )}
      </div>
    </div>
  );
}
