import { describe, expect, it } from 'vitest';

import { ReconnectStrategy } from './reconnect';

describe('ReconnectStrategy', () => {
  it('backs off to the configured ceiling', () => {
    const strategy = new ReconnectStrategy({
      initialDelayMs: 100,
      maxDelayMs: 500,
      factor: 2,
      jitter: false,
    });

    expect([
      strategy.getNextDelay(),
      strategy.getNextDelay(),
      strategy.getNextDelay(),
      strategy.getNextDelay(),
    ]).toEqual([100, 200, 400, 500]);
    expect(strategy.getAttempts()).toBe(4);
  });

  it('starts over after a successful connection', () => {
    const strategy = new ReconnectStrategy({ initialDelayMs: 250, jitter: false });
    strategy.getNextDelay();
    strategy.getNextDelay();
    strategy.reset();

    expect(strategy.getAttempts()).toBe(0);
    expect(strategy.getNextDelay()).toBe(250);
  });
});
