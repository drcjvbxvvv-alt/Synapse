# 前端表單操作設計不一致分析

> 基於 `ui/CLAUDE.md` v1.0 規範，分析日期：2026-04-08
> 範圍：`ui/src` 下已修改或新增的表單相關頁面

---

## 分析範圍

| 檔案 | 類型 |
|---|---|
| `ui/src/pages/network/IngressCreateModal.tsx` | 修改 |
| `ui/src/pages/security/CertificateList.tsx` | 新增 |
| `ui/src/pages/security/SecurityDashboard.tsx` | 修改 |
| `ui/src/pages/workload/DeploymentDetail.tsx` | 修改 |
| `ui/src/pages/workload/WorkloadDetail.tsx` | 修改 |
| `ui/src/pages/cost/CostDashboard.tsx` | 修改 |

---

## 問題一：Modal footer 結構錯誤（高嚴重性）✅ 已修正

**規範 §6.1**：有表單的 Modal 必須用 `footer` prop 傳入按鈕陣列，由 `htmlType="submit"` 觸發表單提交，不得使用預設 `onOk`。

### 修正內容

**`IngressCreateModal.tsx`（L556–571）**

移除 `onOk`、`confirmLoading`、`okText`、`cancelText`，改為自訂 `footer` 陣列，並加上 `destroyOnHide`。

```tsx
// ✅ 修正後
<Modal
  title={t('network:create.ingressTitle')}
  open={visible}
  onCancel={handleCancel}
  width={900}
  destroyOnHide
  footer={[
    <Button key="cancel" onClick={handleCancel}>
      {t('common:actions.cancel')}
    </Button>,
    <Button key="submit" type="primary" loading={loading} onClick={handleSubmit}>
      {t('common:actions.create')}
    </Button>,
  ]}
>
```

---

## 問題二：Form.Item label 非純文字（中嚴重性）✅ 已修正

**規範 §2.2**：`label` 只能是純文字字串，說明資訊移至 `tooltip`，且必須走 i18n。

### 修正內容

**`IngressCreateModal.tsx`**：`label="Ingress Class"` → `label={t('network:create.ingressClassName')}`，`label="Host"` → `label={t('network:create.host')}`

---

## 問題三：i18n 缺失—硬編碼中文字串（高嚴重性）✅ 已修正

**規範 §11.1**：所有面向使用者的文字必須用 `t()` 包裹。

### 修正範圍

| 檔案 | 修正內容 |
|---|---|
| `CertificateList.tsx` | 整頁 i18n 化：StatusTag、column titles/filters、not-installed Alert、tab labels、empty text、card title、refresh button |
| `SecurityDashboard.tsx` | Gatekeeper not-detected Alert 的 `message` 與 `description` |
| `CostDashboard.tsx` | billing message.success/error、occupancy table columns、效率 Alert、散點圖標題、象限說明、雲端帳單 tab label、billing 表單所有 label/placeholder、sync 狀態文字、總覽 card title/buttons/empty |
| `WorkloadDetail.tsx` | `'AI 診斷'` button label |
| `DeploymentDetail.tsx` | `'效能指標'` tab label |
| `IngressCreateModal.tsx` | validation pattern message、`Ingress Class` label、`Host` label |

### 新增 locale keys（3 語言同步）

- `network.json`：`create.ingressClassName`、`create.host`、`create.ingressNamePattern`
- `security.json`：`gatekeeper.notDetectedMsg`、`gatekeeper.notDetectedDesc`、完整 `cert.*` section
- `workload.json`：`actions.aiDiagnose`、`detailTabs.metrics`
- `cost.json`：`tabs.wlEfficiency`、`tabs.capacityTrend`、`occupancy.*`（cpuRequestCol 等）、`billing.*`（saveSuccess 等）

---

## 問題四：Design Token 未使用—硬編碼 style（中嚴重性）✅ 已修正

**規範 §1.2**：禁止硬編碼顏色、間距、圓角、字體大小，一律改用 `token.*`。

### 修正內容

| 檔案 | 修正內容 |
|---|---|
| `DeploymentDetail.tsx` | 加入 `theme.useToken()`；loading div `padding: '100px 0'` → `token.paddingXL * 3`；page wrapper `background: '#f0f2f5'` → `token.colorBgLayout`；header div `background: '#fff'` → `token.colorBgContainer`、`padding` → token、`borderRadius: '8px'` → `token.borderRadius`；`marginBottom: 16` → `token.margin` |
| `SecurityDashboard.tsx` | `SEVERITY_COLORS` 改用 antd preset tag 色名（`red`/`orange`/`gold`/`green`/`default`）取代 hex 值；三個子元件各自加入 `theme.useToken()`；`padding: 40` → `token.paddingXL`（共 3 處）；`#cf1322`/`#52c41a` inline color → `token.colorError`/`token.colorSuccess` |
| `WorkloadDetail.tsx` | 加入 `theme.useToken()`；AI 診斷按鈕 `color: '#fff'` → `token.colorBgContainer`（gradient 色為品牌特殊色，保留） |
| `CertificateList.tsx` | `color: '#8c8c8c'` 已於 P1 重構時移除 |
| `CostDashboard.tsx` | 圖表色陣列加上 `// chart-only` 例外原因註解 |

---

## 問題五：日期格式不統一（中嚴重性）✅ 已修正

**規範 §4.3**：時間顯示統一用 `dayjs`，禁用 `toLocaleString()`。

### 修正內容

`SecurityDashboard.tsx`：加入 `import dayjs from 'dayjs'`，將 3 處 `new Date(v).toLocaleString()` 改為 `dayjs(v).format('YYYY-MM-DD HH:mm')`

---

## 問題六：Input prefix icon 濫用（低嚴重性）✅ 已確認無違規

**規範 §2.3**：只有搜尋框可加 `prefix={<SearchOutlined />}`。

已掃描全部 6 個範圍內檔案，無任何 `prefix={` 用法，無需修正。

---

## 修正優先順序建議

| 優先級 | 問題 | 影響範圍 |
|---|---|---|
| ~~P0~~ | ~~Modal footer 結構錯誤~~ ✅ 已修正 | IngressCreateModal |
| ~~P1~~ | ~~i18n 缺失（影響多語系功能）~~ ✅ 已修正 | 全部 6 個檔案 |
| ~~P1~~ | ~~Form.Item label 硬編碼~~ ✅ 已修正 | IngressCreateModal |
| ~~P2~~ | ~~Design Token 未使用（影響主題切換）~~ ✅ 已修正 | 5 個檔案 |
| ~~P2~~ | ~~日期格式不統一~~ ✅ 已修正 | SecurityDashboard |
| ~~P3~~ | ~~圖表顏色常數例外原因未標注~~ ✅ 已修正（P2 時順帶完成） | CostDashboard |
| ~~P3~~ | ~~Input prefix icon 濫用~~ ✅ 已確認無違規 | — |
