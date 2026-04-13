import React from 'react';
import { Card, Typography, Steps, Button } from 'antd';
import { useTranslation } from 'react-i18next';
import type { Node } from '../../types';
import { useNodeOperations } from './hooks/useNodeOperations';
import {
  OperationSelectStep,
  OperationConfigStep,
  ExecutionProgressStep,
  OperationResultStep,
} from './components/NodeOperationSteps';

const { Title } = Typography;
const { Step } = Steps;

export interface NodeOperationProps {
  clusterId: string;
  selectedNodes: Node[];
  onClose: () => void;
  onSuccess: () => void;
}

const NodeOperations: React.FC<NodeOperationProps> = ({
  clusterId,
  selectedNodes,
  onClose,
  onSuccess,
}) => {
  const { t } = useTranslation(['nodeOps', 'common']);

  const {
    currentStep,
    operationType,
    operationReason,
    drainOptions,
    confirmChecks,
    operationProgress,
    nodeOperationStatus,
    operationResults,
    executionStrategy,
    failureHandling,
    loading,
    handleOperationTypeChange,
    handleNext,
    handlePrevious,
    handleCancel,
    handleFinish,
    handleConfirmChecksChange,
    handleDrainOptionsChange,
    setOperationReason,
    setDrainOptions,
    setExecutionStrategy,
    setFailureHandling,
    getOperationTitle,
    getOperationDescription,
  } = useNodeOperations({ clusterId, selectedNodes, onClose, onSuccess });

  const renderStepContent = () => {
    switch (currentStep) {
      case 0:
        return (
          <OperationSelectStep
            operationType={operationType}
            onOperationTypeChange={handleOperationTypeChange}
            operationReason={operationReason}
            onReasonChange={setOperationReason}
            executionStrategy={executionStrategy}
            onExecutionStrategyChange={setExecutionStrategy}
            failureHandling={failureHandling}
            onFailureHandlingChange={setFailureHandling}
            selectedNodes={selectedNodes}
            operationTitle={getOperationTitle()}
            operationDescription={getOperationDescription()}
          />
        );
      case 1:
        return (
          <OperationConfigStep
            operationType={operationType}
            operationReason={operationReason}
            onReasonChange={setOperationReason}
            drainOptions={drainOptions}
            onDrainOptionsChange={handleDrainOptionsChange}
            onDrainOptionNumberChange={(key, value) =>
              setDrainOptions(prev => ({ ...prev, [key]: value }))
            }
            confirmChecks={confirmChecks}
            onConfirmChecksChange={handleConfirmChecksChange}
            selectedNodes={selectedNodes}
          />
        );
      case 2:
        return (
          <ExecutionProgressStep
            operationTitle={getOperationTitle()}
            operationProgress={operationProgress}
            nodeOperationStatus={nodeOperationStatus}
          />
        );
      case 3:
        return (
          <OperationResultStep
            operationTitle={getOperationTitle()}
            operationResults={operationResults}
          />
        );
      default:
        return null;
    }
  };

  return (
    <div className="node-operations">
      <Card
        title={<Title level={4}>{t('nodeOps:title')}</Title>}
        extra={
          <Button onClick={handleCancel}>
            {t('nodeOps:buttons.cancel')}
          </Button>
        }
      >
        <Steps current={currentStep} style={{ marginBottom: 24 }}>
          <Step title={t('nodeOps:steps.selectOperation')} />
          <Step title={t('nodeOps:steps.configParams')} />
          <Step title={t('nodeOps:steps.executeOperation')} />
          <Step title={t('nodeOps:steps.complete')} />
        </Steps>

        {renderStepContent()}

        <div style={{ marginTop: 24, textAlign: 'right' }}>
          {currentStep > 0 && currentStep < 3 && (
            <Button style={{ marginRight: 8 }} onClick={handlePrevious}>
              {t('nodeOps:buttons.previous')}
            </Button>
          )}
          {currentStep < 2 && (
            <Button type="primary" onClick={handleNext}>
              {t('nodeOps:buttons.next')}
            </Button>
          )}
          {currentStep === 2 && (
            <Button type="primary" onClick={handleNext} loading={loading}>
              {t('nodeOps:buttons.startExecution')}
            </Button>
          )}
          {currentStep === 3 && (
            <Button type="primary" onClick={handleFinish}>
              {t('nodeOps:buttons.finish')}
            </Button>
          )}
        </div>
      </Card>
    </div>
  );
};

export default NodeOperations;
