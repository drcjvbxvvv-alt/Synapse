import React from 'react';
import { useParams } from 'react-router-dom';
import KubectlTerminal from '../../components/KubectlTerminal';

const KubectlTerminalPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  
  if (!id) {
    return <div>集群ID不存在</div>;
  }

  return (
    <div style={{ padding: '24px' }}>
      <KubectlTerminal clusterId={id} />
    </div>
  );
};

export default KubectlTerminalPage;
