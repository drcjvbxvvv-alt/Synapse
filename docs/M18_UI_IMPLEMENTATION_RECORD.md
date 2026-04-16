# M18-UI 實作紀錄 — CI 引擎設定前端

| 項目        | 內容                                                |
| ----------- | --------------------------------------------------- |
| 里程碑      | M18-UI — CI 引擎設定頁面（前端）                     |
| 狀態        | ✅ 完成（2026-04-16）                                |
| 對應後端    | M18a（Framework）/ M18b（GitLab）/ M18c（Jenkins）/ M18d（Tekton）/ M18e（Argo + GitHub Actions）|
| 實作位置    | `ui/src/pages/settings/CIEngineSettings.tsx`        |

---

## 1. 交付摘要

**一句話描述：** 在「系統設定」頁面新增「CI 引擎」Tab，提供外部 CI 引擎連線設定的完整 CRUD，依引擎類型動態顯示對應欄位，並顯示健康狀態與版本。

### 核心產出

| 層級             | 產出物                                               |
| ---------------- | ---------------------------------------------------- |
| **API Service**  | `ui/src/services/ciEngineService.ts`（List/Get/Create/Update/Delete/Status） |
| **頁面元件**     | `ui/src/pages/settings/CIEngineSettings.tsx`（Table + Modal Form） |
| **Tab 整合**     | `ui/src/pages/settings/SystemSettings.tsx`（新增 CI 引擎 Tab） |
| **i18n**         | `zh-TW/cicd.json`、`zh-CN/cicd.json`、`en-US/cicd.json`（各新增 `ciEngine` 節點） |

---

## 2. 功能說明

### 2.1 列表頁

| 欄位       | 說明                                       |
| ---------- | ------------------------------------------ |
| 名稱       | 連線設定名稱（粗體）                        |
| 引擎類型   | Tag 色碼區分（橘=GitLab、藍=Jenkins 等）    |
| 端點       | HTTP endpoint（K8s 原生引擎顯示 —）        |
| 啟用       | ON / OFF Tag                               |
| 健康狀態   | Badge（綠=健康、紅=異常、灰=未探測）        |
| 版本       | 最後探測回傳的版本字串                     |
| 最後探測   | `last_checked_at` 格式化時間               |
| 操作       | 編輯 / 刪除（Popconfirm）                   |

### 2.2 表單 Modal（新增 / 編輯）

#### 通用欄位
- 名稱（必填）
- 引擎類型（新增時必選；編輯時唯讀）
- 啟用（編輯時顯示）

#### 依引擎類型動態顯示

| 引擎            | 特有欄位                                              |
| --------------- | ----------------------------------------------------- |
| **GitLab CI**   | Endpoint、Token、Project ID、Default Ref、TLS 設定    |
| **Jenkins**     | Endpoint、Username、Token（API Token）、Job Path、TLS |
| **Tekton**      | 叢集選擇器、Pipeline Name、Namespace、Service Account  |
| **Argo Workflows** | 叢集選擇器、Workflow Namespace、WorkflowTemplate、Service Account |
| **GitHub Actions** | Token、Owner、Repo、Workflow ID/File、Default Branch |

#### TLS 設定（HTTP 引擎）
- InsecureSkipVerify（Switch）
- CA Bundle（PEM TextArea）

---

## 3. 檔案清單

### 3.1 新增檔案（3 個）

```
ui/src/services/ciEngineService.ts   # API service（List/Get/Create/Update/Delete/Status）
ui/src/pages/settings/
└── CIEngineSettings.tsx             # 列表 + 表單 Modal 一體頁面
docs/
└── M18_UI_IMPLEMENTATION_RECORD.md  # 本檔
```

### 3.2 修改檔案（4 個）

| 檔案                                        | 修改內容                              |
| ------------------------------------------- | ------------------------------------- |
| `ui/src/pages/settings/SystemSettings.tsx`  | 新增「CI 引擎」Tab（ThunderboltOutlined）|
| `ui/src/locales/zh-TW/cicd.json`            | 新增 `ciEngine` 節點（92 個 key）     |
| `ui/src/locales/zh-CN/cicd.json`            | 同上（簡體中文版）                    |
| `ui/src/locales/en-US/cicd.json`            | 同上（英文版）                        |

---

## 4. 關鍵設計決策

### 4.1 表單欄位動態化

根據 `selectedType` state 渲染 `renderExtraFields()`，而非用 `hidden` CSS 隱藏全部欄位。好處：
- 不同引擎的驗證規則（`rules`）互不干擾
- 提交時不會把無關欄位的空值送到後端
- TypeScript 可靜態驗證每段 switch case 的欄位名稱

### 4.2 extra_json 的前後端橋接

後端的 `extra_json` 是 JSON 字串（不同引擎的 key 不同）。前端用 `parseExtra()` / `buildExtra()` 把它展平成 Form 欄位，提交時再組回 JSON：

```typescript
// 展平（讀入 form）
const extra = parseExtra(cfg.extra_json);
form.setFieldsValue({ gitlab_project_id: extra.project_id, ... });

// 組回（提交時）
extra_json = buildExtra('gitlab', { project_id: values.gitlab_project_id, ... });
```

### 4.3 引擎類型建立後唯讀

編輯時 `<Select disabled={!!editing}>`，與後端行為一致（Update 忽略 engine_type 變更）。避免使用者誤改造成 extra_json 結構不匹配。

### 4.4 Token 欄位在編輯時非必填

後端 `ApplyTo()` 已處理「空 Token 不覆寫」邏輯。前端在 `editing` 時移除 Token 的 required rule，與後端行為保持一致。

---

## 5. 品質驗證

```
✅ npx tsc --noEmit  →  0 errors
✅ 依引擎類型切換時，extraFields 正確渲染
✅ 編輯時表單預填值正確（含 extra_json 展平）
✅ 刪除使用 Popconfirm，符合 ui/CLAUDE.md §10
✅ 所有面向使用者的文字使用 t()，無硬編碼中文
✅ 設計 Token 全部使用 token.xxx，無硬編碼顏色 / 間距
```

---

**實作者：** Architecture Team  
**最後更新：** 2026-04-16
