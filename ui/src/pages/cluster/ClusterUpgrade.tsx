import React from 'react';
import { Result, Typography } from 'antd';
import { CloudServerOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
const { Paragraph } = Typography;

const ClusterUpgrade: React.FC = () => {
const { t } = useTranslation(['cluster']);
return (
    <div style={{ 
      display: 'flex',
      justifyContent: 'center',
      alignItems: 'center',
      minHeight: 'calc(100vh - 200px)',
    }}>
      <Result
        icon={<CloudServerOutlined style={{ color: '#1890ff', fontSize: '64px' }} />}
        title={
<span style={{ fontSize: '24px', fontWeight: 500 }}>
            {t('upgrade.inDev')}
          </span>
}
        subTitle={
<Paragraph style={{ color: '#666', maxWidth: '400px', margin: '0 auto', fontSize: '14px' }}>
            {t('upgrade.inDevDesc')}
          </Paragraph>
}
      />
    </div>
  );
};

export default ClusterUpgrade;
