# GOGG Web 前端

GOGG 英雄排行榜的现代 React + TypeScript 前端。

## 项目结构

```
web/
├── src/
│   ├── assets/                 # 静态资源
│   │   ├── images/            # 图片资源（英雄、装备等）
│   │   │   ├── heroes/        # 英雄图标
│   │   │   ├── items/         # 装备图标
│   │   │   └── common/        # 通用 UI 图标
│   │   └── styles/            # 全局和实用样式
│   │
│   ├── components/            # 可复用的 React 组件
│   │   ├── Layout/            # 布局组件（Header、Footer、MainLayout）
│   │   ├── UI/                # 通用 UI 组件（FiltersPanel、Table 等）
│   │   └── index.ts           # 组件导出
│   │
│   ├── pages/                 # 页面级组件
│   │   ├── Rankings/          # 排行榜页面
│   │   └── Home/              # （未来）首页
│   │
│   ├── services/              # API 客户端和服务
│   │   └── api.ts             # API 调用
│   │
│   ├── hooks/                 # 自定义 React hooks
│   │   └── useRankings.ts     # 排行榜数据获取 hook
│   │
│   ├── types/                 # TypeScript 类型定义
│   │   └── index.ts           # 集中式类型导出
│   │
│   ├── utils/                 # 工具函数和常量
│   │   └── constants.ts       # 应用常量
│   │
│   ├── App.tsx                # 根组件
│   └── main.tsx               # 应用入口
│
├── index.html                 # HTML 模板
├── package.json               # 项目依赖
├── tsconfig.json              # TypeScript 配置
├── vite.config.ts             # Vite 配置
└── README.md                  # 项目说明
```

## 主要特性

- ✅ **TypeScript**：应用级别的完整类型安全
- 🎨 **CSS Modules**：局部样式作用域，防止冲突
- 📦 **路径别名**：使用 `@` 前缀进行清晰的导入
- 🎯 **基于组件**：模块化和可维护的架构
- 🔌 **自定义 Hooks**：可复用的逻辑
- 📱 **响应式设计**：移动端友好的布局

## 快速开始

### 安装

```bash
npm install
```

### 开发

```bash
npm run dev
```

在 `http://localhost:5173` 启动开发服务器

### 构建

```bash
npm run build
```

在 `dist/` 目录生成优化后的生产构建

### 类型检查

```bash
npm run type-check
```

验证 TypeScript 不生成代码

## 路径别名

项目使用路径别名进行清晰的导入：

- `@/` → `src/`
- `@components/` → `src/components/`
- `@pages/` → `src/pages/`
- `@services/` → `src/services/`
- `@hooks/` → `src/hooks/`
- `@types/` → `src/types/`
- `@utils/` → `src/utils/`
- `@assets/` → `src/assets/`

在导入中使用这些别名：

```typescript
// ✅ 推荐
import { useRankings } from "@hooks/useRankings";
import { FiltersPanel } from "@components/UI";

// ❌ 避免
import { useRankings } from "../../hooks/useRankings";
```

## 添加新页面

1. 在 `src/pages/{PageName}` 中创建文件夹
2. 添加 `{PageName}.tsx` 组件
3. 从 `src/pages/{PageName}/index.ts` 导出
4. 在 `src/App.tsx` 中更新路由（或使用 React Router 配置路由）

示例：

```typescript
// src/pages/Heroes/Heroes.tsx
export function Heroes() {
  return <div>英雄页面</div>;
}

// src/pages/Heroes/index.ts
export { Heroes } from "./Heroes";
```

## 添加静态资源

### 图片

1. 将图片放在 `src/assets/images/` 下的相应文件夹中
   - 英雄图标：`src/assets/images/heroes/`
   - 装备图标：`src/assets/images/items/`
   - UI 图标：`src/assets/images/common/`

2. 在组件中导入：

```typescript
import ahriImage from "@assets/images/heroes/ahri.png";

export function HeroCard() {
  return <img src={ahriImage} alt="Ahri" />;
}
```

### 样式

- 全局样式：`src/assets/styles/index.css`
- 组件样式：`{ComponentName}.module.css`（与组件放在一起）

## API 集成

API 调用集中在 `src/services/api.ts` 中：

```typescript
// src/services/api.ts
export async function fetchChampionRankings(filters) {
  // ...
}

// 在组件或 hooks 中使用
import { fetchChampionRankings } from "@services/api";
```

## 状态管理

目前使用 React hooks 进行状态管理。对于更大的应用，考虑使用：

- **TanStack Query**：管理服务器状态
- **Zustand**：管理客户端状态
- **Redux/Redux Toolkit**：管理复杂状态树

## 测试

使用以下工具添加测试：

- **Vitest**：快速单元测试
- **React Testing Library**：组件测试
- **Playwright/Cypress**：端到端测试

## 浏览器支持

- Chrome（最新版）
- Firefox（最新版）
- Safari（最新版）
- Edge（最新版）

---

祝你编码愉快！🚀
