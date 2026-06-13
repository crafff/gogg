# 迁移指南：TypeScript + 优化结构

本指南说明了项目如何从 JavaScript 重构为 TypeScript 并采用优化的文件结构。

## 主要变化

### 1. 语言：JavaScript → TypeScript

**之前：**
- `main.jsx`、`App.jsx` - JavaScript 文件
- 没有类型安全
- Props 和 state 类型在运行时推断

**之后：**
- `main.tsx`、`App.tsx` - TypeScript 文件
- 完整的类型安全和显式类型定义
- 更好的 IDE 支持和自动补全

### 2. 文件结构

**之前：**
```
src/
├── App.jsx
├── main.jsx
└── styles.css
```

**之后：**
```
src/
├── components/      # 可复用组件
├── pages/          # 页面组件
├── services/       # API 层
├── hooks/          # 自定义 hooks
├── types/          # TypeScript 类型
├── utils/          # 常量和工具函数
├── assets/         # 静态资源
├── App.tsx
└── main.tsx
```

### 3. 类型定义

新的 `src/types/index.ts` 定义所有共享类型：

```typescript
export interface RankingItem {
  [key: string]: unknown;
}

export interface RankingsResponse {
  items: RankingItem[];
}

export type Position = "" | "TOP" | "JUNGLE" | "MIDDLE" | "BOTTOM" | "UTILITY";

export interface RankingsFilters {
  position: Position;
  limit: number;
  minGames: number;
}
```

### 4. API 服务

API 调用移到 `src/services/api.ts`：

**之前：**
```javascript
// 在组件中内联 fetch
const res = await fetch(`/api/rankings/champions?${query}`);
```

**之后：**
```typescript
// 集中式服务
import { fetchChampionRankings } from "@services/api";

const data = await fetchChampionRankings(filters);
```

### 5. 自定义 Hooks

逻辑提取到 `src/hooks/useRankings.ts`：

**之前：**
```javascript
// 所有状态逻辑都在组件中
const [items, setItems] = useState([]);
const [loading, setLoading] = useState(false);
// ... 组件的其余部分
```

**之后：**
```typescript
// 自定义 hook
const { items, loading, error } = useRankings(filters);
```

### 6. 组件组织

组件模块化，使用 CSS Modules 进行样式隔离：

```
src/components/
├── UI/
│   ├── FiltersPanel.tsx
│   ├── FiltersPanel.module.css
│   ├── RankingsTable.tsx
│   ├── RankingsTable.module.css
│   └── index.ts
└── Layout/
    ├── Header.tsx
    ├── Header.module.css
    └── index.ts
```

### 7. 路径别名

使用路径别名代替相对导入：

**之前：**
```typescript
import { FiltersPanel } from "../../components/UI/FiltersPanel";
import { useRankings } from "../../hooks/useRankings";
```

**之后：**
```typescript
import { FiltersPanel } from "@components/UI";
import { useRankings } from "@hooks/useRankings";
```

## 安装和设置

### 1. 安装依赖

```bash
npm install
```

这将安装 TypeScript、类型定义和其他工具。

### 2. 环境配置

项目配置有：
- `tsconfig.json` - TypeScript 编译器配置
- `vite.config.ts` - Vite 打包工具配置
- 两者中都有路径别名

### 3. 开发

```bash
npm run dev
```

### 4. 构建

```bash
npm run build
```

先进行类型检查，然后构建生产版本。

### 5. 仅进行类型检查

```bash
npm run type-check
```

## 主要优势

### 类型安全
- 在编译时捕捉错误
- 更好的 IDE 自动补全
- 代码是自我文档化的

### 可维护性
- 关注点清晰分离
- 易于添加新页面/组件
- 可扩展的结构支持未来增长

### 资源组织
- 专用的 `src/assets/` 文件夹
- `images/` 子目录分类（英雄、装备等）
- 易于添加更多静态资源

### 未来就绪
- 结构支持多页面（使用 React Router）
- 内置 CSS Module 支持
- 环境变量支持（通过 Vite）

## 添加新功能

### 新页面

1. 创建：`src/pages/YourPage/YourPage.tsx`
2. 导出：`src/pages/YourPage/index.ts`
3. 在 `src/App.tsx` 中添加路由

### 新组件

1. 创建：`src/components/YourCategory/YourComponent.tsx`
2. 可选添加：`src/components/YourCategory/YourComponent.module.css`
3. 从分类的 `index.ts` 导出

### 新静态图片

1. 添加到：`src/assets/images/{category}/`
2. 导入：`import image from "@assets/images/{category}/name.png"`

### 新的 API 调用

1. 添加函数到：`src/services/api.ts`
2. 如需要在 `src/types/index.ts` 中更新类型
3. 在组件或 hooks 中使用

## 后续步骤

需要时推荐添加：

1. **路由**：添加 `react-router-dom` 进行多页面导航
2. **状态管理**：添加 `TanStack Query` 管理服务器状态
3. **测试**：添加 `vitest` 和 `@testing-library/react`
4. **样式**：考虑使用 `tailwindcss` 或 `scss` 实现更复杂的样式
5. **代码检查**：添加 `eslint` 和 `prettier` 保证代码一致性

## 故障排查

### 导入不能解析？
- 检查 `vite.config.ts` 中的别名定义
- 确保 `tsconfig.json` 有正确的 `paths` 配置
- 配置更改后重启开发服务器

### TypeScript 错误？
- 运行 `npm run type-check` 查看所有类型问题
- 检查 `src/types/index.ts` 中的类型定义
- 使用 `as unknown` 作为最后手段（尽可能避免）

### 构建失败？
- 先运行 `npm run type-check` 检查类型错误
- 清除 `dist/` 文件夹并重新构建
- 检查缺失的依赖：`npm install`

---

如有问题或疑问，请参考 [PROJECT_STRUCTURE.md](./PROJECT_STRUCTURE.md) 文件或相应服务/组件的文档。
