import React from 'react';
import { Spin, Menu, Row, Col, Tag, Divider, Empty } from 'antd';
import { useTranslation } from 'react-i18next';
import type { ContainerTabProps } from './containerTypes';
import { useContainerTab } from './hooks/useContainerTab';
import { ContainerBasicSection } from './sections/ContainerBasicSection';
import { ContainerLifecycleSection } from './sections/ContainerLifecycleSection';
import { ContainerHealthSection } from './sections/ContainerHealthSection';
import { ContainerEnvSection } from './sections/ContainerEnvSection';
import { ContainerVolumeSection } from './sections/ContainerVolumeSection';

const ContainerTab: React.FC<ContainerTabProps> = (props) => {
  const { t } = useTranslation(['workload', 'common']);
  const {
    loading,
    spec,
    selectedContainer,
    setSelectedContainer,
    selectedSection,
    setSelectedSection,
  } = useContainerTab(props);

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: '50px 0' }}>
        <Spin tip={t('common:messages.loading')} />
      </div>
    );
  }

  if (!spec || !spec.template?.spec?.containers || spec.template.spec.containers.length === 0) {
    return <Empty description={t('container.noContainers')} />;
  }

  const containers = spec.template.spec.containers;
  const currentContainer = containers.find(c => c.name === selectedContainer);
  const volumes = spec.template.spec.volumes;

  const menuItems = [
    { key: 'basic',     label: t('container.menu.basic') },
    { key: 'lifecycle', label: t('container.menu.lifecycle') },
    { key: 'health',    label: t('container.menu.health') },
    { key: 'env',       label: t('container.menu.env') },
    { key: 'volume',    label: t('container.menu.volume') },
  ];

  const renderSection = () => {
    if (!currentContainer) return null;
    switch (selectedSection) {
      case 'basic':
        return <ContainerBasicSection container={currentContainer} t={t} />;
      case 'lifecycle':
        return <ContainerLifecycleSection container={currentContainer} t={t} />;
      case 'health':
        return <ContainerHealthSection container={currentContainer} t={t} />;
      case 'env':
        return <ContainerEnvSection container={currentContainer} t={t} />;
      case 'volume':
        return <ContainerVolumeSection container={currentContainer} volumes={volumes} t={t} />;
      default:
        return null;
    }
  };

  return (
    <div>
      {containers.length > 1 && (
        <>
          <div style={{ marginBottom: 16 }}>
            <span style={{ marginRight: 8 }}>{t('container.containerList')}</span>
            {containers.map(container => (
              <Tag
                key={container.name}
                color={container.name === selectedContainer ? 'blue' : 'default'}
                style={{ cursor: 'pointer', marginBottom: 8 }}
                onClick={() => setSelectedContainer(container.name)}
              >
                {container.name}
              </Tag>
            ))}
          </div>
          <Divider />
        </>
      )}

      <Row gutter={16}>
        <Col span={4}>
          <Menu
            mode="inline"
            selectedKeys={[selectedSection]}
            items={menuItems}
            onClick={({ key }) => setSelectedSection(key)}
          />
        </Col>
        <Col span={20}>
          {renderSection()}
        </Col>
      </Row>
    </div>
  );
};

export default ContainerTab;
