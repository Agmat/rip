"use client";

import { useEffect, useRef, useState } from "react";
import Image from "next/image";
import type { SetSummary } from "@/lib/types";

// iOS gates DeviceOrientationEvent behind a user-gesture permission prompt;
// no other browser exposes this static method.
type IOSDeviceOrientationEvent = typeof DeviceOrientationEvent & {
  requestPermission?: () => Promise<"granted" | "denied">;
};

// Android exposes deviceorientation without a permission prompt - start
// listening right away there. iOS needs requestGyro() from a tap first.
function androidHasGyro() {
  if (typeof window === "undefined" || typeof DeviceOrientationEvent === "undefined") {
    return false;
  }
  const hasPointer = window.matchMedia("(hover: hover)").matches;
  const needsPrompt =
    typeof (DeviceOrientationEvent as IOSDeviceOrientationEvent).requestPermission ===
    "function";
  return !hasPointer && !needsPrompt;
}

export default function Pack({ set, tearing }: { set: SetSummary; tearing: boolean }) {
  const ref = useRef<HTMLDivElement>(null);
  const [idle, setIdle] = useState(true);
  const [gyroActive, setGyroActive] = useState(androidHasGyro);

  function setHighlight(x: number, y: number) {
    ref.current?.style.setProperty("--mx", x.toFixed(3));
    ref.current?.style.setProperty("--my", y.toFixed(3));
  }

  useEffect(() => {
    if (!gyroActive) return;
    function onOrientation(e: DeviceOrientationEvent) {
      if (e.beta == null || e.gamma == null) return;
      setIdle(false);
      setHighlight(
        Math.min(1, Math.max(0, (e.gamma + 45) / 90)),
        Math.min(1, Math.max(0, e.beta / 90)),
      );
    }
    window.addEventListener("deviceorientation", onOrientation);
    return () => window.removeEventListener("deviceorientation", onOrientation);
  }, [gyroActive]);

  async function requestGyro() {
    if (typeof DeviceOrientationEvent === "undefined") return;
    const DOE = DeviceOrientationEvent as IOSDeviceOrientationEvent;
    if (typeof DOE.requestPermission !== "function") return;
    try {
      const state = await DOE.requestPermission();
      if (state === "granted") setGyroActive(true);
    } catch {
      // denied or unsupported - keep the idle sweep
    }
  }

  function handlePointerMove(e: React.PointerEvent<HTMLDivElement>) {
    const rect = e.currentTarget.getBoundingClientRect();
    setIdle(false);
    setHighlight((e.clientX - rect.left) / rect.width, (e.clientY - rect.top) / rect.height);
  }

  function handlePointerLeave() {
    setIdle(true);
    setHighlight(0.5, 0.5);
  }

  return (
    <div
      ref={ref}
      className={`pack${tearing ? " tearing" : ""}`}
      onPointerMove={handlePointerMove}
      onPointerLeave={handlePointerLeave}
      onPointerDown={requestGyro}
    >
      <div className="pack-crimp top" />
      <div className="pack-body">
        <Image
          className="pack-icon"
          src={`https://svgs.scryfall.io/sets/${set.code.toLowerCase()}.svg`}
          alt=""
          width={44}
          height={44}
          unoptimized
        />
        <p className="pack-name">{set.name}</p>
        <p className="pack-type">Play Booster, 14 cards</p>
      </div>
      <div className="pack-crimp bottom" />
      <div className={`pack-sheen${idle ? " idle" : ""}`} />
    </div>
  );
}
