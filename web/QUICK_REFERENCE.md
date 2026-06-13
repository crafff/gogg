# 快速参考指南 - GOGG 前端

## 文件结构速览

```
src/
├── main.tsx              # 入口点
├── App.tsx               # 根组件
├── components/           # 可复用的 UI 组件
├── pages/                # 页面级组件
├── services/             # API 层
├── hooks/                # 自定义 React hooks
├── types/                # TypeScript 类型定义
├── utils/                # 常量和工具函数
└── assets/               # 静态文件（图片、样式）
```

## 常见任务

### 1. 添加新页面

**步骤 1：** 创建页面文件夹和文件
```bash
mkdir -p src/pages/MyPage
```

**步骤 2：** 创建页面组件
```typescript
// src/pages/MyPage/MyPage.tsx
export function MyPage() {
  return <h1>我的页面</h1>;
}
```

**步骤 3：** 从 index 导出
```typescript
// src/pages/MyPage/index.ts
export { MyPage } from "./MyPage";
```

**步骤 4：** 添加到 App.tsx 路由（使用 React Router 时）

---

### 2. 添加新组件

**步骤 1：** 创建组件文件
```typescript
// src/components/UI/Button.tsx
interface ButtonProps {
  label: string;
  onClick: () => void;
}

export function Button({ label, onClick }: ButtonProps) {
  return <button onClick={onClick}>{label}</button>;
}
```

**步骤 2：** 添加组件样式（可选）
```css
/* src/components/UI/Button.module.css */
button {
  padding: 0.5rem 1rem;
  background: #667eea;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}
```

**步骤 3：** 从分类 index 导出
```typescript
// src/components/UI/index.ts
export { Button } from "./Button";
```

---

### 3. 添加新的 API 端点

**步骤 1：** 添加函数到 api.ts
```typescript
// src/services/api.ts
export async function fetchHeroDetails(heroId: string) {
  const response = await fetch(`${API_BASE}/heroes/${heroId}`);
  return response.json();
}
```

**步骤 2：** 如需要，添加类型
```typescript
// src/types/index.ts
export interface Hero {
  id: string;
  name: string;
  // ...
}
```

**步骤 3：** 在组件或 hook 中使用
```typescript
import { fetchHeroDetails } from "@services/api";

// 在 hook 或 useEffect 中
const data = await fetchHeroDetails("ahri");
```

---

### 4. 添加静态图片

**步骤 1：** 将图片放在相应文件夹
```
src/assets/images/
├── heroes/ahri.png
├── items/trinity-force.png
└── common/search-icon.svg
```

**步骤 2：** 在组件中导入
```typescript
import ahriImage from "@assets/images/heroes/ahri.png";

export function HeroCard() {
  return <img src={ahriImage} alt="Ahri 英雄" />;
}
```

---

### 5. 创建自定义 Hook

**步骤 1：** 创建 hook 文件
```typescript
// src/hooks/useHeroData.ts
import { useEffect, useState } from "react";
import { fetchHeroDetails } from "@services/api";

export function useHeroData(heroId: string) {
  const [hero, setHero] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    async function load() {
      try {
        setLoading(true);
        setError(null);
        const data = await fetchHeroDetails(heroId);
        setHero(data);
      } catch (err) {
        setError(err);
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [heroId]);

  return { hero, loading, error };
}
```

**步骤 2：** 从 hooks/index.ts 导出
```typescript
export { useHeroData } from "./useHeroData";
```

**步骤 3：** 在组件中使用
```typescript
import { useHeroData } from "@hooks/useHeroData";

function MyComponent() {
  const { hero, loading, error } = useHeroData("ahri");
  // ...
}
```

---

## 导入模式

### 路径别名
```typescript
// ✅ 推荐 - 清晰且一致
import { FiltersPanel } from "@components/UI";
import { useRankings } from "@hooks/useRankings";
import type { RankingItem } from "@types";
import { DEFAULT_FILTERS } from "@utils/constants";
import logo from "@assets/images/logo.png";

// ❌ 避免 - 相对路径难以维护
import { FiltersPanel } from "../../../components/UI/FiltersPanel";
```

### 类型导入
```typescript
// 推荐 - 使用 'type' 关键字导入类型
import type { RankingsFilters } from "@types";

// 在组件中使用
interface MyComponentProps extends RankingsFilters {
  onApply: () => void;
}
```

---

## 组件模式

### 带 Props 的函数组件
```typescript
interface MyComponentProps {
  title: string;
  isActive: boolean;
  onToggle: (value: boolean) => void;
}

export function MyComponent({ title, isActive, onToggle }: MyComponentProps) {
  return (
    <div>
      <h2>{title}</h2>
      <button onClick={() => onToggle(!isActive)}>
        {isActive ? "激活" : "未激活"}
      </button>
    </div>
  );
}
```

### 使用 CSS Modules
```typescript
import styles from "./MyComponent.module.css";

export function MyComponent() {
  return <div className={styles.container}>内容</div>;
}
```

```css
/* MyComponent.module.css */
.container {
  padding: 1rem;
  background: white;
}
```

---

## 可用命令

```bash
# 开发
npm run dev              # 在 :5173 启动开发服务器

# 生产
npm run build           # 构建优化的包
npm run preview         # 本地预览生产构建

# 质量检查
npm run type-check      # 检查 TypeScript 错误
```

---

## 类型定义检查清单

添加新功能时，在 `src/types/index.ts` 中定义类型：

- ✅ API 响应类型
- ✅ 组件 Props 类型
- ✅ 表单数据类型
- ✅ 筛选/查询类型
- ✅ 自定义 hook 返回类型

```typescript
// 好的示例
export interface ApiResponse<T> {
  data: T;
  error?: string;
  timestamp: number;
}

export interface Champion {
  id: string;
  name: string;
  role: "TOP" | "JUNGLE" | "MID" | "ADC" | "SUPPORT";
}
```

---

## 调试技巧

### TypeScript 错误
```bash
npm run type-check    # 查看所有类型错误及文件/行号信息
```

### 构建问题
1. 清除缓存：`rm -rf dist node_modules`
2. 重新安装：`npm install`
3. 重新构建：`npm run build`

### 导入问题
- 验证 `vite.config.ts` 和 `tsconfig.json` 中的路径别名
- 配置更改后重启开发服务器
- 检查确切的文件夹/文件名称（Linux 区分大小写）

---

## 下一步：使用 React Router 的多页面设置

当准备添加多个页面时：

```bash
npm install react-router-dom
```

然后更新 `src/App.tsx`：

```typescript
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { Rankings } from "@pages/Rankings";
import { Home } from "@pages/Home";

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/rankings" element={<Rankings />} />
      </Routes>
    </BrowserRouter>
  );
}
```

---

## 文件命名规范

| 类型 | 格式 | 示例 |
|------|--------|---------|
| 组件 | PascalCase | `Button.tsx`、`Header.tsx` |
| 页面 | PascalCase | `Rankings.tsx`、`Home.tsx` |
| Hooks | 带 `use` 的 camelCase | `useRankings.ts`、`useFetch.ts` |
| 样式 | `{Name}.module.css` | `Button.module.css` |
| 类型 | PascalCase + Type 后缀 | `RankingsFilters`（在 types/index.ts 中） |
| 常量 | UPPER_SNAKE_CASE | `DEFAULT_LIMIT = 20` |
| 工具函数 | camelCase 或常量 | `formatNumber()`、`DATE_FORMAT` |
| 文件夹 | 小写或 camelCase | `components/`、`src/` |

---

## 资源

- [TypeScript 文档](https://www.typescriptlang.org/docs/)
- [React 文档](https://react.dev/)
- [Vite 文档](https://vitejs.dev/)
- 查看 `PROJECT_STRUCTURE.md` 了解详细概述
- 查看 `MIGRATION_GUIDE.md` 了解 TypeScript 转换详情

---

**最后更新：** 2024
**项目：** GOGG 前端
