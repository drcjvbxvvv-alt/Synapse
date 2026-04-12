# 長文本展示設計規範

## 概述

本規範定義了如何在表格、列表等數據展示組件中安全地處理長文本，確保：
- ✓ 每一行數據高度保持一致（不會跑版）
- ✓ 使用者能夠快速查看完整內容
- ✓ UI 簡潔不被額外按鈕佔用空間
- ✓ 無縫複製體驗

## 使用場景

### 必須使用本規範的情況

1. **表格列** - 任何可能超過 80 字元的文本列
   - 消息、日誌內容
   - 資源名稱、Pod 名稱
   - 來源地址、URL

2. **詳情展示** - 抽屜、Modal 中的長文本字段
   - 事件訊息
   - 資源詳情
   - 完整描述

3. **列表項** - Card、List 中可能超行的內容

### 不需要使用本規範的情況

- 固定長度文本（如狀態標籤）
- 單個關鍵字（如命名空間）
- 已知短文本（< 20 字元）

## 核心設計原則

### 1. 固定高度 - 確保不跑版

```tsx
// ❌ 禁止：使用 maxHeight 讓文本自動換行，會導致行高不一致
<Text style={{ maxHeight: 300, overflow: 'auto' }}>
  {longText}
</Text>

// ✅ 正確：使用 ellipsis 截斷，保持固定高度
<Text ellipsis>{longText}</Text>
```

### 2. 省略號顯示 - 視覺截斷

**強制顯示「...」省略號，不換行：**

```tsx
// 使用 Ant Design 的 ellipsis 屬性
<Text
  ellipsis  // 自動加上 text-overflow: ellipsis
>
  {longText}
</Text>

// 或使用原生 CSS
<div style={{
  whiteSpace: 'nowrap',        // 不換行
  overflow: 'hidden',          // 隱藏超出部分
  textOverflow: 'ellipsis'     // 顯示「...」
}}>
  {longText}
</div>

// 多行截斷（常用於 Card 或 Table）
<div style={{
  display: '-webkit-box',
  WebkitLineClamp: 2,          // 最多 2 行
  WebkitBoxOrient: 'vertical',
  overflow: 'hidden',
  textOverflow: 'ellipsis'
}}>
  {longText}
</div>
```

### 3. 懸停卡片 - 展示完整內容

使用 **Popover** 組件（不是 Tooltip）：
- Tooltip：只適合簡短提示（< 50 字）
- Popover：適合長文本，支援內部按鈕和格式化

### 4. 視覺提示 - 指示用戶可互動

```tsx
// 使用藍色和 cursor:pointer 提示可點擊
<Text
  ellipsis            // 顯示「...」省略號
  style={{
    cursor: 'pointer',
    color: '#1890ff'  // 主題色
  }}
>
  {longText}
</Text>
```

## 實現方式

### 方案 1：使用全局輔助函數（推薦）

在 `src/pages/logs/columns.tsx` 中定義並複用：

```typescript
import { Popover, Text, Button } from 'antd';
import { CopyOutlined } from '@ant-design/icons';
import type { TFunction } from 'i18next';

// 長文本懸停卡片（帶「...」省略號）
const TextPopover: React.FC<{ content: string; t: TFunction }> = ({
  content,
  t
}) => (
  <Popover
    content={
      <div style={{
        maxWidth: 500,           // 卡片寬度
        wordBreak: 'break-all',  // 字詞截斷
        maxHeight: 300,          // 卡片最大高度
        overflow: 'auto'         // 內容過長時捲動
      }}>
        <div style={{ marginBottom: 8 }}>
          {content}
        </div>
        <Button
          type="primary"
          size="small"
          icon={<CopyOutlined />}
          onClick={() => navigator.clipboard.writeText(content)}
        >
          {t('common:actions.copy')}
        </Button>
      </div>
    }
    title={t('logs:center.preview')}  // 或其他適當的標題
  >
    <Text
      ellipsis  // ⭐ 顯示「...」省略號
      style={{
        cursor: 'pointer',
        color: '#1890ff'
      }}
    >
      {content}
    </Text>
  </Popover>
);
```

### 方案 2：直接在列定義中使用

```typescript
// 表格列定義
const columns: ColumnsType<RecordType> = [
  {
    title: t('table.message'),
    dataIndex: 'message',
    render: (message: string) => (
      <Popover
        content={
          <div style={{
            maxWidth: 500,
            wordBreak: 'break-all',
            maxHeight: 300,
            overflow: 'auto'
          }}>
            <div style={{ marginBottom: 8 }}>{message}</div>
            <Button
              type="primary"
              size="small"
              icon={<CopyOutlined />}
              onClick={() => navigator.clipboard.writeText(message)}
            >
              {t('common:actions.copy')}
            </Button>
          </div>
        }
        title={t('logs:center.preview')}
      >
        <Text
          ellipsis
          style={{ cursor: 'pointer', color: '#1890ff' }}
        >
          {message}
        </Text>
      </Popover>
    ),
  },
  // ... 其他列
];
```

## 樣式規範

### Popover 卡片內容

```tsx
// 卡片容器樣式
<div style={{
  maxWidth: 500,         // 根據內容調整，範圍 400-800px
  wordBreak: 'break-all', // 強制字詞截斷
  maxHeight: 300,        // 防止卡片過高，範圍 200-400px
  overflow: 'auto'       // 超出時可捲動
}}>
  {/* 內容 */}
</div>
```

### 文本樣式

```tsx
// ⭐ 表格/列表中的文本（單行 + 「...」省略號）
<Text
  ellipsis                           // 自動加上 text-overflow: ellipsis
  style={{
    cursor: 'pointer',               // 鼠標變為手形
    color: '#1890ff'                // 主題色，表示可互動
  }}
>
  {content}
</Text>

// 或使用原生 CSS
<span style={{
  display: 'block',
  whiteSpace: 'nowrap',              // ⭐ 不換行
  overflow: 'hidden',                // ⭐ 隱藏超出部分
  textOverflow: 'ellipsis',          // ⭐ 顯示「...」
  cursor: 'pointer',
  color: '#1890ff'
}}>
  {content}
</span>

// 詳情展示中的文本（多行截斷）
<Paragraph
  style={{
    display: '-webkit-box',
    WebkitLineClamp: 2,              // ⭐ 最多 2 行後「...」
    WebkitBoxOrient: 'vertical',
    overflow: 'hidden',              // ⭐ 隱藏超出部分
    textOverflow: 'ellipsis',        // ⭐ 尾部省略號
    wordBreak: 'break-all',          // 字詞截斷
    cursor: 'pointer',
    color: '#1890ff'
  }}
>
  {content}
</Paragraph>
```

### 複製按鈕

```tsx
<Button
  type="primary"           // 或 type="text"
  size="small"            // 保持卡片內按鈕尺寸
  icon={<CopyOutlined />} // 使用複製圖標
  onClick={() => navigator.clipboard.writeText(content)}
>
  {t('common:actions.copy')}
</Button>
```

## i18n 鍵值

確保以下 i18n 鍵值已定義：

```json
// logs.json 中添加
{
  "center": {
    "preview": "Preview"  // 或 "預覽" / "预览"
  }
}

// common.json 中確認存在
{
  "actions": {
    "copy": "Copy"  // 或 "複製" / "复制"
  },
  "messages": {
    "success": "Success"  // 複製成功時顯示
  }
}
```

## 實現檢查清單

在任何表格/列表中展示長文本時，確保：

```
□ 文本超過內容寬度時顯示「...」省略號
  - 單行：使用 ellipsis 或 text-overflow: ellipsis
  - 多行：使用 WebkitLineClamp + text-overflow: ellipsis
□ 使用 <Text ellipsis /> 或 CSS text-overflow 固定高度
□ 沒有在表格行中使用 maxHeight/overflow 導致行高變化（省略號除外）
□ 不使用 wordBreak 在表格中（會導致換行和高度不一致）
□ 添加 cursor: pointer 和藍色文字提示可互動
□ 使用 Popover 而不是 Tooltip
□ Popover 卡片設定 maxWidth 和 maxHeight
□ 卡片內容支持 wordBreak: 'break-all'
□ 提供複製按鈕在 Popover 中
□ 導入並使用 CopyOutlined 圖標
□ i18n 鍵值已定義（preview, copy）
□ 測試行高一致性（所有行應為同樣高度）
□ 測試「...」省略號正確顯示
```

## 常見錯誤

### ❌ 錯誤 1：在表格行中換行

```tsx
// 錯誤 - 導致行高不一致
render: (text) => (
  <div style={{ wordBreak: 'break-all' }}>
    {text}
  </div>
)
```

**修正**：使用 `ellipsis` 截斷，不在表格中換行。

### ❌ 錯誤 2：使用 Tooltip 顯示長文本

```tsx
// 錯誤 - Tooltip 不適合長文本
<Tooltip title={longText}>
  <Text ellipsis>{longText}</Text>
</Tooltip>
```

**修正**：使用 `Popover` 代替 Tooltip。

### ❌ 錯誤 3：沒有視覺提示

```tsx
// 錯誤 - 用戶不知道可以點擊
<Text ellipsis>{longText}</Text>
```

**修正**：添加 `cursor: pointer` 和藍色文字。

### ❌ 錯誤 4：沒有省略號

```tsx
// 錯誤 - 長文本直接顯示，超出表格邊界
<Text>{longText}</Text>

// 錯誤 - 文本換行，導致行高不一致
<Text style={{ wordBreak: 'break-all' }}>
  {longText}
</Text>
```

**修正**：必須使用 `ellipsis` 或 CSS `text-overflow: ellipsis` 顯示「...」。

### ❌ 錯誤 5：複製按鈕在表格中

```tsx
// 錯誤 - 佔用空間，影響高度一致性
render: (text) => (
  <Space>
    <Text>{text}</Text>
    <Button onClick={() => copy(text)} />
  </Space>
)
```

**修正**：將複製功能放在 Popover 卡片中。

## 性能考慮

1. **Popover 延遲加載**：Popover 內容在打開時才渲染，不影響初始加載
2. **複製操作**：使用原生 `navigator.clipboard.writeText()`，無需三方庫
3. **大列表優化**：
   - 使用虛擬滾動（`Table` 的 `virtual` 屬性）
   - Popover 會自動位置調整，無需額外計算

## 測試方法

### 視覺測試

1. 打開包含長文本的表格
2. 檢查所有行高度是否一致
3. 滑鼠懸停文本，確認 Popover 出現
4. 點擊複製按鈕，驗證複製功能

### 邊界情況

- 超短文本（< 10 字元）：不應觸發省略號
- 超長文本（> 1000 字元）：應能正常顯示和複製
- 特殊字符：應正確換行和複製
- 超多行數據：表格應保持整齊排列

## 相關文件

- 實現參考：`src/pages/logs/columns.tsx` - `TextPopover` 函數
- EventLogs 詳情：`src/pages/logs/EventLogs.tsx` - Popover 使用示例
- i18n 配置：`src/locales/{en-US,zh-CN,zh-TW}/logs.json`

## 更新歷史

| 版本 | 日期 | 更新內容 |
|------|------|--------|
| 1.0  | 2026-04-12 | 初版，定義長文本展示規範 |

---

**請在開發前端時嚴格遵循本規範，確保 UI 的一致性和用戶體驗。**
