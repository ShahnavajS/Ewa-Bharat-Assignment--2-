export interface ReconnectConfig {
  initialDelayMs?: number;
  maxDelayMs?: number;
  factor?: number;
  jitter?: boolean;
}

/**
 * Exponential backoff tracker for graceful WebSocket reconnections.
 */
export class ReconnectStrategy {
  private readonly initialDelayMs: number;
  private readonly maxDelayMs: number;
  private readonly factor: number;
  private readonly jitter: boolean;
  private attempts: number = 0;

  constructor(config: ReconnectConfig = {}) {
    this.initialDelayMs = config.initialDelayMs ?? 1000;
    this.maxDelayMs = config.maxDelayMs ?? 10000;
    this.factor = config.factor ?? 2;
    this.jitter = config.jitter ?? true;
  }

  public getNextDelay(): number {
    const delay = Math.min(
      this.initialDelayMs * Math.pow(this.factor, this.attempts),
      this.maxDelayMs
    );
    this.attempts++;

    if (!this.jitter) {
      return delay;
    }

    // Add +/- 20% randomized jitter to prevent thundering herds
    const jitterMultiplier = 0.8 + Math.random() * 0.4;
    return Math.floor(delay * jitterMultiplier);
  }

  public reset(): void {
    this.attempts = 0;
  }

  public getAttempts(): number {
    return this.attempts;
  }
}
