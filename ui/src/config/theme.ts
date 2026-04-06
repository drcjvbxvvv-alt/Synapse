import type { ThemeConfig } from 'antd';

// Synapse Design Token
// 所有 Ant Design v5 元件的視覺定義集中於此，
// 透過 App.tsx 的 ConfigProvider theme 注入，禁止在 CSS 中使用 .ant-* 選擇器覆蓋。
export const synapseTheme: ThemeConfig = {
  token: {
    // 主色
    colorPrimary: '#006eff',
    colorPrimaryHover: '#1a7aff',
    colorPrimaryActive: '#0056cc',

    // 背景
    colorBgLayout: '#f5f7fa',
    colorBgContainer: '#ffffff',
    colorBgElevated: '#ffffff',

    // 邊框
    colorBorder: '#e8eaec',
    colorBorderSecondary: '#f0f1f2',

    // 文字
    colorText: '#333333',
    colorTextSecondary: '#666666',
    colorTextTertiary: '#999999',

    // 圓角
    borderRadius: 8,
    borderRadiusLG: 12,
    borderRadiusSM: 6,

    // 陰影
    boxShadow: '0 1px 4px 0 rgba(0, 0, 0, 0.08)',
    boxShadowSecondary: '0 4px 12px 0 rgba(0, 0, 0, 0.12)',

    // 字型（繁體中文優先）
    fontFamily:
      "-apple-system, BlinkMacSystemFont, 'PingFang TC', 'Hiragino Sans TC', " +
      "'Microsoft JhengHei', Arial, sans-serif",
    fontSize: 14,
    lineHeight: 1.5,

    // 間距
    padding: 16,
    paddingLG: 24,
    paddingSM: 12,
  },
  components: {
    Layout: {
      headerBg: '#ffffff',
      siderBg: '#ffffff',
      bodyBg: '#f5f7fa',
      headerHeight: 60,
      headerPadding: '0 24px',
    },
    Menu: {
      itemBorderRadius: 8,
      itemMarginInline: 8,
      itemPaddingInline: 12,
      itemSelectedBg: '#006eff',
      itemSelectedColor: '#ffffff',
      itemHoverBg: '#f0f6ff',
      itemHoverColor: '#006eff',
      itemColor: '#333333',
      subMenuItemBg: '#ffffff',
      groupTitleColor: '#999999',
      groupTitleFontSize: 12,
      // 緊湊高度（取代 .compact-menu CSS 注入）
      itemHeight: 36,
    },
    Button: {
      borderRadius: 8,
      primaryShadow: '0 2px 4px 0 rgba(0, 110, 255, 0.3)',
      fontWeight: 500,
    },
    Card: {
      borderRadius: 12,
      boxShadow: '0 1px 4px 0 rgba(0, 0, 0, 0.08)',
      headerFontSize: 16,
      headerFontSizeSM: 14,
    },
    Table: {
      headerBg: '#f8f9fa',
      borderRadius: 12,
      headerColor: '#333333',
      headerSortActiveBg: '#f0f1f2',
      rowHoverBg: '#f8f9fa',
    },
    Input: {
      borderRadius: 8,
      activeShadow: '0 0 0 2px rgba(0, 110, 255, 0.1)',
    },
    Select: {
      borderRadius: 8,
    },
    Tag: {
      borderRadius: 6,
      fontSize: 12,
      fontWeightStrong: 500,
    },
    Pagination: {
      borderRadius: 8,
      itemActiveBg: '#006eff',
    },
    Statistic: {
      titleFontSize: 14,
      contentFontSize: 28,
    },
    Modal: {
      borderRadius: 12,
    },
    Drawer: {
      borderRadius: 12,
    },
    Badge: {
      statusSize: 8,
    },
  },
};
