import React from 'react';
import { ConnectionStatus } from '../types';

interface ConnectionBadgeProps {
  status: ConnectionStatus;
  labelPrefix?: string;
}

export const ConnectionBadge: React.FC<ConnectionBadgeProps> = ({
  status,
  labelPrefix = 'WebSocket',
}) => {
  const getStatusText = () => {
    switch (status) {
      case 'connected':
        return 'Connected';
      case 'connecting':
        return 'Connecting...';
      case 'reconnecting':
        return 'Reconnecting...';
      case 'disconnected':
        return 'Disconnected';
      case 'error':
        return 'Error';
    }
  };

  return (
    <div className="status-badge" title={`Connection status: ${status}`}>
      <span className={`status-dot ${status}`} />
      <span>
        {labelPrefix}: <strong>{getStatusText()}</strong>
      </span>
    </div>
  );
};
