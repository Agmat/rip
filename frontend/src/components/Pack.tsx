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
  const [gyroActive, setGyroActive] = useState(androidHasGyro);

  function setTilt(x: number, y: number) {
    ref.current?.style.setProperty("--mx", x.toFixed(3));
    ref.current?.style.setProperty("--my", y.toFixed(3));
  }

  useEffect(() => {
    if (!gyroActive) return;
    function onOrientation(e: DeviceOrientationEvent) {
      if (e.beta == null || e.gamma == null) return;
      setTilt(
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
      // denied or unsupported - pack just stays flat
    }
  }

  function handlePointerMove(e: React.PointerEvent<HTMLDivElement>) {
    const rect = e.currentTarget.getBoundingClientRect();
    setTilt((e.clientX - rect.left) / rect.width, (e.clientY - rect.top) / rect.height);
  }

  function handlePointerLeave() {
    setTilt(0.5, 0.5);
  }

  if (!set.pack_image_url) {
    // ponytail: text-only fallback; revisit only if a real set ships with
    // no TCGplayer listing to resolve a pack photo from.
    return (
      <div className="pack pack-fallback">
        <p className="pack-name">{set.name}</p>
      </div>
    );
  }

  return (
    <div
      ref={ref}
      className={`pack${tearing ? " tearing" : ""}`}
      onPointerMove={handlePointerMove}
      onPointerLeave={handlePointerLeave}
      onPointerDown={requestGyro}
    >
      <div className="pack-half top">
        <Image
          src={set.pack_image_url}
          alt={`${set.name} Play Booster pack`}
          fill
          sizes="240px"
          unoptimized
          priority
        />
      </div>
      <div className="pack-half bottom">
        <Image src={set.pack_image_url} alt="" fill sizes="240px" unoptimized aria-hidden />
      </div>
    </div>
  );
}
