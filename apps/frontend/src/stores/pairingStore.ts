// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createSignal } from "solid-js";
import type { PairingRequestEvent } from "@/hooks";

const [pendingPairingsQueue, setPendingPairingsQueue] = createSignal<PairingRequestEvent[]>([]);

// Callback set by AuthModals so that Header can open the modal for a specific request.
let _openPairingRequest: ((req: PairingRequestEvent) => void) | null = null;

const setOpenPairingRequestHandler = (fn: (req: PairingRequestEvent) => void) => {
  _openPairingRequest = fn;
};

const openPairingRequest = (req: PairingRequestEvent) => {
  _openPairingRequest?.(req);
};

export { pendingPairingsQueue, setPendingPairingsQueue, setOpenPairingRequestHandler, openPairingRequest };
