// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createSignal } from "solid-js";

const [wsConnected, setWsConnected] = createSignal(false);

export const useWsConnection = () => {
  return {
    isConnected: wsConnected,
    setConnected: (connected: boolean) => setWsConnected(connected),
  };
};

export { wsConnected };
