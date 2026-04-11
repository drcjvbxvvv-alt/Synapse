import type {
  RolloutStrategyConfig,
  CanaryStrategyConfig,
  BlueGreenStrategyConfig,
  CanaryStep,
} from '../../types/workload';

// ==================== Argo Rollout 策略構建 ====================

export const buildCanaryStrategy = (canary: CanaryStrategyConfig | undefined): Record<string, unknown> => {
  if (!canary) {
    return {
      canary: {
        steps: [
          { setWeight: 20 },
          { pause: { duration: '10m' } },
          { setWeight: 50 },
          { pause: { duration: '10m' } },
          { setWeight: 80 },
          { pause: { duration: '10m' } },
        ],
      },
    };
  }

  const canarySpec: Record<string, unknown> = {};

  if (canary.steps && canary.steps.length > 0) {
    const rawSteps: Array<Record<string, unknown>> = [];

    canary.steps.forEach((step: CanaryStep) => {
      if (step.setWeight !== undefined) {
        rawSteps.push({ setWeight: step.setWeight });
      }

      if (step.pause !== undefined) {
        if (step.pause.duration) {
          rawSteps.push({ pause: { duration: step.pause.duration } });
        } else {
          rawSteps.push({ pause: {} });
        }
      }

      if (step.setCanaryScale) {
        rawSteps.push({ setCanaryScale: step.setCanaryScale });
      }

      if (step.analysis) {
        rawSteps.push({ analysis: step.analysis });
      }
    });

    if (rawSteps.length > 0) {
      canarySpec.steps = rawSteps;
    }
  }

  if (canary.maxSurge) canarySpec.maxSurge = canary.maxSurge;
  if (canary.maxUnavailable) canarySpec.maxUnavailable = canary.maxUnavailable;

  if (canary.stableService) canarySpec.stableService = canary.stableService;
  if (canary.canaryService) canarySpec.canaryService = canary.canaryService;

  if (canary.trafficRouting) {
    const trafficRouting: Record<string, unknown> = {};
    if (canary.trafficRouting.nginx?.stableIngress) {
      trafficRouting.nginx = {
        stableIngress: canary.trafficRouting.nginx.stableIngress,
        ...(canary.trafficRouting.nginx.annotationPrefix && {
          annotationPrefix: canary.trafficRouting.nginx.annotationPrefix,
        }),
      };
    }
    if (canary.trafficRouting.istio) {
      trafficRouting.istio = canary.trafficRouting.istio;
    }
    if (canary.trafficRouting.alb) {
      trafficRouting.alb = canary.trafficRouting.alb;
    }
    if (Object.keys(trafficRouting).length > 0) {
      canarySpec.trafficRouting = trafficRouting;
    }
  }

  if (canary.analysis) canarySpec.analysis = canary.analysis;
  if (canary.canaryMetadata) canarySpec.canaryMetadata = canary.canaryMetadata;
  if (canary.stableMetadata) canarySpec.stableMetadata = canary.stableMetadata;
  if (canary.antiAffinity) canarySpec.antiAffinity = canary.antiAffinity;

  return { canary: canarySpec };
};

export const buildBlueGreenStrategy = (blueGreen: BlueGreenStrategyConfig | undefined): Record<string, unknown> => {
  if (!blueGreen || !blueGreen.activeService) {
    return {
      blueGreen: {
        activeService: 'my-app-active',
        previewService: 'my-app-preview',
        autoPromotionEnabled: false,
      },
    };
  }

  const blueGreenSpec: Record<string, unknown> = {
    activeService: blueGreen.activeService,
  };

  if (blueGreen.previewService) blueGreenSpec.previewService = blueGreen.previewService;
  if (blueGreen.autoPromotionEnabled !== undefined) {
    blueGreenSpec.autoPromotionEnabled = blueGreen.autoPromotionEnabled;
  }
  if (blueGreen.autoPromotionSeconds !== undefined) {
    blueGreenSpec.autoPromotionSeconds = blueGreen.autoPromotionSeconds;
  }
  if (blueGreen.scaleDownDelaySeconds !== undefined) {
    blueGreenSpec.scaleDownDelaySeconds = blueGreen.scaleDownDelaySeconds;
  }
  if (blueGreen.scaleDownDelayRevisionLimit !== undefined) {
    blueGreenSpec.scaleDownDelayRevisionLimit = blueGreen.scaleDownDelayRevisionLimit;
  }
  if (blueGreen.previewReplicaCount !== undefined) {
    blueGreenSpec.previewReplicaCount = blueGreen.previewReplicaCount;
  }

  if (blueGreen.previewMetadata) blueGreenSpec.previewMetadata = blueGreen.previewMetadata;
  if (blueGreen.activeMetadata) blueGreenSpec.activeMetadata = blueGreen.activeMetadata;
  if (blueGreen.antiAffinity) blueGreenSpec.antiAffinity = blueGreen.antiAffinity;
  if (blueGreen.prePromotionAnalysis) blueGreenSpec.prePromotionAnalysis = blueGreen.prePromotionAnalysis;
  if (blueGreen.postPromotionAnalysis) blueGreenSpec.postPromotionAnalysis = blueGreen.postPromotionAnalysis;

  return { blueGreen: blueGreenSpec };
};

export const buildRolloutStrategy = (rolloutStrategy: RolloutStrategyConfig | undefined): Record<string, unknown> => {
  if (!rolloutStrategy) {
    return buildCanaryStrategy(undefined);
  }

  if (rolloutStrategy.type === 'BlueGreen') {
    return buildBlueGreenStrategy(rolloutStrategy.blueGreen);
  }

  return buildCanaryStrategy(rolloutStrategy.canary);
};
