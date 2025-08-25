# 认证系统更新说明

## 🔐 Token认证机制

根据后端登录接口返回格式 `{"token": "xxxx"}` 和后续请求需要添加 `Prism-Token: xxxx` header 的要求，完成了前端认证系统的更新。

## 📝 更新内容

### 1. 类型定义更新 (`src/types/index.ts`)

```typescript
// 登录请求接口（字段从username改为account）
export interface LoginRequest {
    account: string;
    password: string;
}

// 新增登录响应接口
export interface LoginResponse {
    token: string;
}
```

### 2. Token管理工具 (`src/utils/token.ts`)

创建了专门的token管理工具，负责：
- ✅ 本地存储token (`localStorage`)
- ✅ 获取/设置/删除token
- ✅ 检查token是否存在

```typescript
export const tokenManager = {
  getToken(): string | null
  setToken(token: string): void
  removeToken(): void
  hasToken(): boolean
}
```

### 3. API服务更新 (`src/services/api.ts`)

**请求拦截器增强：**
- ✅ 自动在所有请求header中添加 `Prism-Token: xxxx`
- ✅ 登录接口除外（避免循环）
- ✅ 401响应自动清除过期token

**登录API更新：**
- ✅ 返回类型改为 `LoginResponse`
- ✅ 登录成功后自动保存token到本地
- ✅ 登出时自动清除本地token

```typescript
// 通用请求函数自动添加token header
async function fetchApi<T>(endpoint: string, options: RequestInit = {}) {
    const headers: Record<string, string> = {
        'Content-Type': 'application/json',
        ...options.headers as Record<string, string>,
    };

    // 添加token header（登录接口除外）
    const token = tokenManager.getToken();
    if (token && endpoint !== '/login') {
        headers['Prism-Token'] = token;
    }
    
    // ... 其他逻辑
}

// 登录API保存token
login: async (data: LoginRequest) => {
    const response = await fetchApi<LoginResponse>('/login', {
        method: 'POST',
        body: JSON.stringify(data),
    });
    
    // 登录成功后保存token
    if (response.data?.token) {
        tokenManager.setToken(response.data.token);
    }
    
    return response;
}
```

### 4. 认证逻辑更新

**Layout组件 (`src/components/Layout.tsx`)：**
- ✅ 只有存在token时才获取用户信息
- ✅ token过期时自动跳转登录页
- ✅ 退出登录时确保清除本地token

**Login组件 (`src/components/Login.tsx`)：**
- ✅ 登录字段从`username`改为`account`
- ✅ 验证登录响应中的token
- ✅ 成功后跳转到仪表盘

## 🔄 认证流程

### 登录流程
1. 用户输入账号密码
2. 调用 `/api/login` 接口
3. 后端返回 `{"token": "xxxx"}`
4. 前端自动保存token到localStorage
5. 跳转到仪表盘页面

### 请求流程  
1. 前端发起API请求
2. 自动从localStorage获取token
3. 在header中添加 `Prism-Token: xxxx`
4. 发送请求到后端
5. 如果401未授权，自动清除token并跳转登录页

### 退出流程
1. 用户点击退出登录
2. 调用 `/api/account/logout` 接口
3. 无论成功与否，清除本地token
4. 跳转到登录页面

## 🛡️ 安全特性

- ✅ **自动token管理**: 登录成功自动保存，过期自动清除
- ✅ **请求拦截**: 所有需要认证的请求自动添加token header
- ✅ **状态同步**: token状态与UI状态实时同步
- ✅ **错误处理**: 401错误自动重定向到登录页
- ✅ **内存安全**: 退出登录时彻底清除认证信息

## 🔧 使用说明

### 开发环境
```bash
cd frontend
npm install
npm run dev
```

### 生产构建
```bash
npm run build
```

### 注意事项
- 后端登录接口需要返回格式：`{"token": "xxxx"}`
- 所有需要认证的接口都会自动添加 `Prism-Token: xxxx` header
- token存储在localStorage中，页面刷新后仍然有效
- 401响应会自动清除token并跳转登录页

## ✅ 测试验证

- ✅ TypeScript编译通过
- ✅ 前端项目构建成功  
- ✅ 认证流程逻辑完整
- ✅ 与后端API接口匹配

---

🔐 **认证系统已完全适配后端token机制，可以正常进行用户认证和API调用！**
