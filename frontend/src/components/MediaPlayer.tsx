import React, { useRef, useEffect, useState } from 'react';
import { MediaItem, PlaybackMode } from '../types';

interface MediaPlayerProps {
  media: MediaItem | null;
  mode: PlaybackMode;
  elapsedSeconds: number;
  remainingSeconds: number;
  isMuted?: boolean;
}

export const MediaPlayer: React.FC<MediaPlayerProps> = ({
  media,
  mode,
  elapsedSeconds,
  remainingSeconds,
  isMuted = true,
}) => {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const [hasError, setHasError] = useState<boolean>(false);
  const [errorMessage, setErrorMessage] = useState<string>('');

  // Reset error when media changes
  useEffect(() => {
    setHasError(false);
    setErrorMessage('');
  }, [media?.id, media?.url]);

  // Video playback time synchronization with drift threshold of 0.5s
  useEffect(() => {
    if (media?.type === 'video' && videoRef.current) {
      const video = videoRef.current;

      const targetTime = () => (
        Number.isFinite(video.duration) && video.duration > 0
          ? elapsedSeconds % video.duration
          : elapsedSeconds
      );

      const handleLoadedMetadata = () => {
        const target = targetTime();
        if (target > 0 && Math.abs(video.currentTime - target) > 0.5) {
          video.currentTime = target;
        }
        video.play().catch(() => {
          // Autoplay policy might require mute (already handled via muted prop)
        });
      };

      if (video.readyState >= HTMLMediaElement.HAVE_METADATA) {
        handleLoadedMetadata();
      } else {
        video.addEventListener('loadedmetadata', handleLoadedMetadata, { once: true });
      }

      // Synchronize drift if greater than 0.5 seconds
      const target = targetTime();
      if (Math.abs(video.currentTime - target) > 0.5) {
        video.currentTime = target;
      }
    }
  }, [media?.id, media?.type, elapsedSeconds]);

  // 1. Inactive State (Empty playlist or inactive mode)
  if (mode === 'inactive' || !media) {
    return (
      <div className="player-fallback inactive">
        <div className="fallback-icon">OFFLINE</div>
        <div className="fallback-title">No Media Configured</div>
        <div className="fallback-desc">This display window playlist is currently inactive or empty.</div>
      </div>
    );
  }

  // 2. Error Fallback (broken media URL or unsupported format)
  if (hasError) {
    return (
      <div className="player-fallback error">
        <div className="fallback-icon">ERROR</div>
        <div className="fallback-title">Media Load Failed</div>
        <div className="fallback-desc">{errorMessage || `Could not load ${media.type}: "${media.title}"`}</div>
        <div className="fallback-url" title={media.url}>{media.url}</div>
      </div>
    );
  }

  // 3. Explicit Blank Media (Intentionally configured standby screen)
  if (media.type === 'blank') {
    return (
      <div className="player-blank-screen">
        <div className="blank-watermark">
          <span className="blank-dot" />
          <span>DISPLAY STANDBY</span>
        </div>
      </div>
    );
  }

  // 4. Video Player
  if (media.type === 'video') {
    return (
      <div className="player-media-container">
        <video
          ref={videoRef}
          src={media.url}
          className="player-video-element"
          autoPlay
          muted={isMuted}
          playsInline
          loop
          controls={false}
          onError={() => {
            setHasError(true);
            setErrorMessage(`Failed to decode video: "${media.title}"`);
          }}
        />
        <div className="player-time-badge">
          {elapsedSeconds}s / {elapsedSeconds + remainingSeconds}s
        </div>
      </div>
    );
  }

  // 5. Image Player
  return (
    <div className="player-media-container">
      <img
        src={media.url}
        alt={media.title}
        className="player-image-element"
        onError={() => {
          setHasError(true);
          setErrorMessage(`Failed to load image: "${media.title}"`);
        }}
      />
      <div className="player-time-badge">
        {elapsedSeconds}s / {elapsedSeconds + remainingSeconds}s
      </div>
    </div>
  );
};
