import { AgentEvent } from '../types';

export class WebSocketClient {
  private ws: WebSocket | null = null;
  private listeners: ((event: AgentEvent) => void)[] = [];
  private statusListeners: ((connected: boolean) => void)[] = [];
  private reconnectTimer: number | null = null;

  connect() {
    if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) {
      return;
    }

    const loc = window.location;
    const protocol = loc.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = loc.host || 'localhost:8787';
    const url = `${protocol}//${host}/ws`;

    this.ws = new WebSocket(url);

    this.ws.onopen = () => {
      this.notifyStatus(true);
    };

    this.ws.onclose = () => {
      this.notifyStatus(false);
      this.scheduleReconnect();
    };

    this.ws.onerror = () => {
      this.notifyStatus(false);
    };

    this.ws.onmessage = (event) => {
      try {
        const parsed = JSON.parse(event.data);
        this.notifyListeners(parsed);
      } catch (e) {
        console.error('Failed to parse WS message:', e);
      }
    };
  }

  private scheduleReconnect() {
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.reconnectTimer = window.setTimeout(() => {
      this.connect();
    }, 2000);
  }

  onEvent(cb: (event: AgentEvent) => void) {
    this.listeners.push(cb);
    return () => {
      this.listeners = this.listeners.filter((l) => l !== cb);
    };
  }

  onStatus(cb: (connected: boolean) => void) {
    this.statusListeners.push(cb);
    return () => {
      this.statusListeners = this.statusListeners.filter((l) => l !== cb);
    };
  }

  private notifyListeners(ev: AgentEvent) {
    for (const l of this.listeners) l(ev);
  }

  private notifyStatus(connected: boolean) {
    for (const l of this.statusListeners) l(connected);
  }

  send(data: any) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data));
    }
  }

  disconnect() {
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }
}

export const wsClient = new WebSocketClient();
