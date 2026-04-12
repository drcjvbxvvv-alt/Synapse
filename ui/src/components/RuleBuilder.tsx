import React from 'react';
import {
  Button,
  Input,
  Select,
  Row,
  Col,
  theme,
} from 'antd';
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

// ─── Types ─────────────────────────────────────────────────────────────────

export interface LabelPair {
  key: string;
  value: string;
}

export interface RulePort {
  protocol: string;
  port: string;
}

export interface RuleState {
  id: number;
  type: 'all' | 'pod' | 'namespace' | 'ipblock';
  selector: LabelPair[];
  cidr: string;
  ports: RulePort[];
}

// ─── Factory ───────────────────────────────────────────────────────────────

let _ruleCounter = 1000;

/** Create a blank rule with a unique id. */
export const newRule = (): RuleState => ({
  id: ++_ruleCounter,
  type: 'all',
  selector: [],
  cidr: '',
  ports: [],
});

// ─── Component ─────────────────────────────────────────────────────────────

export interface RuleBuilderProps {
  rules: RuleState[];
  onChange: (rules: RuleState[]) => void;
  /** 'ingress' shows "Source type" label; 'egress' shows "Destination type". */
  direction: 'ingress' | 'egress';
}

/**
 * Reusable NetworkPolicy rule list editor.
 * Supports pod-selector, namespace-selector, IP block, and allow-all peers,
 * plus arbitrary TCP/UDP/SCTP port entries per rule.
 */
export function RuleBuilder({ rules, onChange, direction }: RuleBuilderProps) {
  const { t } = useTranslation('network');
  const { token } = theme.useToken();

  const update = (id: number, patch: Partial<RuleState>) =>
    onChange(rules.map(r => (r.id === id ? { ...r, ...patch } : r)));

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: token.marginSM }}>
      {rules.map((rule, idx) => (
        <div
          key={rule.id}
          style={{
            border: `1px solid ${token.colorBorder}`,
            borderRadius: token.borderRadius,
            padding: token.paddingSM,
          }}
        >
          {/* Rule header */}
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: token.marginXS }}>
            <span style={{ fontWeight: 600, fontSize: token.fontSizeSM }}>
              {t('networkpolicy.form.ruleLabel', { index: idx + 1 })}
            </span>
            {rules.length > 1 && (
              <Button
                size="small"
                danger
                icon={<DeleteOutlined />}
                onClick={() => onChange(rules.filter(r => r.id !== rule.id))}
              />
            )}
          </div>

          {/* Peer type + CIDR */}
          <Row gutter={token.marginSM} style={{ marginBottom: token.marginXS }}>
            <Col span={12}>
              <div style={{ fontSize: token.fontSizeSM, marginBottom: 4, color: token.colorTextSecondary }}>
                {direction === 'ingress'
                  ? t('networkpolicy.form.sourceType')
                  : t('networkpolicy.form.destType')}
              </div>
              <Select
                value={rule.type}
                onChange={v => update(rule.id, { type: v })}
                style={{ width: '100%' }}
                options={[
                  { value: 'all', label: t('networkpolicy.form.allowAll') },
                  { value: 'pod', label: t('networkpolicy.form.podSelectorType') },
                  { value: 'namespace', label: t('networkpolicy.form.namespaceSelectorType') },
                  { value: 'ipblock', label: t('networkpolicy.form.ipBlock') },
                ]}
              />
            </Col>
            {rule.type === 'ipblock' && (
              <Col span={12}>
                <div style={{ fontSize: token.fontSizeSM, marginBottom: 4, color: token.colorTextSecondary }}>
                  {t('networkpolicy.form.cidr')}
                </div>
                <Input
                  value={rule.cidr}
                  onChange={e => update(rule.id, { cidr: e.target.value })}
                  placeholder={t('networkpolicy.form.cidrPlaceholder')}
                />
              </Col>
            )}
          </Row>

          {/* Label selector (pod or namespace) */}
          {(rule.type === 'pod' || rule.type === 'namespace') && (
            <div style={{ marginBottom: token.marginXS }}>
              <div style={{ fontSize: token.fontSizeSM, marginBottom: 4, color: token.colorTextSecondary }}>
                {t('networkpolicy.form.labelSelector')}
              </div>
              {rule.selector.map((pair, pi) => (
                <Row key={pi} gutter={8} style={{ marginBottom: 4 }}>
                  <Col span={10}>
                    <Input
                      placeholder={t('networkpolicy.form.keyPlaceholder')}
                      value={pair.key}
                      onChange={e =>
                        update(rule.id, {
                          selector: rule.selector.map((p, i) =>
                            i === pi ? { ...p, key: e.target.value } : p
                          ),
                        })
                      }
                    />
                  </Col>
                  <Col span={10}>
                    <Input
                      placeholder={t('networkpolicy.form.valuePlaceholder')}
                      value={pair.value}
                      onChange={e =>
                        update(rule.id, {
                          selector: rule.selector.map((p, i) =>
                            i === pi ? { ...p, value: e.target.value } : p
                          ),
                        })
                      }
                    />
                  </Col>
                  <Col span={4}>
                    <Button
                      danger
                      size="small"
                      icon={<DeleteOutlined />}
                      onClick={() =>
                        update(rule.id, { selector: rule.selector.filter((_, i) => i !== pi) })
                      }
                    />
                  </Col>
                </Row>
              ))}
              <Button
                size="small"
                icon={<PlusOutlined />}
                onClick={() =>
                  update(rule.id, { selector: [...rule.selector, { key: '', value: '' }] })
                }
              >
                {t('networkpolicy.form.addLabel')}
              </Button>
            </div>
          )}

          {/* Ports */}
          <div>
            <div style={{ fontSize: token.fontSizeSM, marginBottom: 4, color: token.colorTextSecondary }}>
              {t('networkpolicy.form.ports')}
            </div>
            {rule.ports.map((p, pi) => (
              <Row key={pi} gutter={8} style={{ marginBottom: 4 }}>
                <Col span={8}>
                  <Select
                    value={p.protocol}
                    onChange={v =>
                      update(rule.id, {
                        ports: rule.ports.map((pp, i) => (i === pi ? { ...pp, protocol: v } : pp)),
                      })
                    }
                    style={{ width: '100%' }}
                    options={[
                      { value: 'TCP', label: 'TCP' },
                      { value: 'UDP', label: 'UDP' },
                      { value: 'SCTP', label: 'SCTP' },
                    ]}
                  />
                </Col>
                <Col span={12}>
                  <Input
                    placeholder={t('networkpolicy.form.portPlaceholder')}
                    value={p.port}
                    onChange={e =>
                      update(rule.id, {
                        ports: rule.ports.map((pp, i) => (i === pi ? { ...pp, port: e.target.value } : pp)),
                      })
                    }
                  />
                </Col>
                <Col span={4}>
                  <Button
                    danger
                    size="small"
                    icon={<DeleteOutlined />}
                    onClick={() =>
                      update(rule.id, { ports: rule.ports.filter((_, i) => i !== pi) })
                    }
                  />
                </Col>
              </Row>
            ))}
            <Button
              size="small"
              icon={<PlusOutlined />}
              onClick={() =>
                update(rule.id, { ports: [...rule.ports, { protocol: 'TCP', port: '' }] })
              }
            >
              {t('networkpolicy.form.addPort')}
            </Button>
          </div>
        </div>
      ))}

      <Button
        type="dashed"
        icon={<PlusOutlined />}
        onClick={() => onChange([...rules, newRule()])}
      >
        {t('networkpolicy.form.addRule')}
      </Button>
    </div>
  );
}
