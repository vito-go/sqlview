# SQLView

<div align="center">

🗄️ **轻量级、可嵌入的 Go 应用数据库查看器**

[![Go Report Card](https://goreportcard.com/badge/github.com/vito-go/sqlview)](https://goreportcard.com/report/github.com/vito-go/sqlview)
[![GoDoc](https://godoc.org/github.com/vito-go/sqlview?status.svg)](https://godoc.org/github.com/vito-go/sqlview)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

[English](README.md) | [简体中文](README_CN.md)

</div>

---

## ✨ 特性

- 🚀 **零依赖** - 单个 HTML 文件嵌入，无需外部资源
- 🔌 **简单集成** - 2 行代码即可挂载到任何 HTTP 服务器
- 🗄️ **多数据库支持** - PostgreSQL、MySQL、SQLite
- 🎨 **现代化 UI** - 简洁、响应式的 Web 界面
- 🔍 **智能排序** - 自动按 ID、updated_at 等字段排序
- 📊 **表检查器** - 查看表结构、索引、DDL、统计信息
- 💾 **CSV 导出** - 导出查询结果（客户端实现）
- 🔒 **只读模式** - 仅允许 SELECT 查询，确保安全
- 🎯 **右键菜单** - 右键快速操作
- 🌐 **多数据库模式** - 在不同数据库间快速切换

## 📸 截图

### 主界面
浏览数据库、表，并使用现代化的 Web UI 运行查询。

### 右键菜单
右键点击任意表即可快速操作：查看数据、表结构、索引、DDL 或统计信息。

### 表统计信息
查看行数、表大小和索引信息。

## 🚀 快速开始

### 安装

```bash
go get github.com/vito-go/sqlview
```

### 基本用法

```go
package main

import (
    "database/sql"
    "log"
    "net/http"

    "github.com/vito-go/sqlview"
    _ "github.com/lib/pq" // PostgreSQL 驱动
)

func main() {
    // 连接到数据库
    db, _ := sql.Open("postgres",
        "postgres://user:pass@localhost:5432/mydb?sslmode=disable")

    // 创建 SQLView 实例（自动检测数据库类型）
    viewer := sqlview.New(db, "/sqlview")

    // 挂载到 HTTP 服务器
    mux := http.NewServeMux()
    viewer.Mount(mux)

    log.Println("SQLView 运行于 http://localhost:8080/sqlview")
    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

就是这么简单！在浏览器中打开 http://localhost:8080/sqlview 即可。

## 📚 文档

### 创建查看器

#### 方式 1：自动检测数据库类型（推荐）

```go
viewer := sqlview.New(db, "/sqlview")
```

SQLView 会自动检测 PostgreSQL、MySQL 或 SQLite。

#### 方式 2：多数据库模式

通过提供不包含数据库名称的 DSN 来启用多数据库浏览：

```go
// PostgreSQL：省略数据库名
dsn := "postgres://user:pass@localhost:5432/?sslmode=disable"
viewer := sqlview.NewWithDSN(db, "/sqlview", dsn, "postgres")

// MySQL：省略数据库名
dsn := "user:pass@tcp(host:3306)/?parseTime=true"
viewer := sqlview.NewWithDSN(db, "/sqlview", dsn, "mysql")
```

### 挂载到 HTTP 服务器

```go
// 方式 1：挂载到现有的 ServeMux
mux := http.NewServeMux()
viewer.Mount(mux)

// 方式 2：获取独立的 handler
handler := viewer.Handler()
http.ListenAndServe(":8080", handler)
```

### 添加中间件（认证、日志等）

```go
func authMiddleware(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
    token := r.Header.Get("Authorization")
    if token == "" {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return r, false // 停止处理
    }
    return r, true // 继续
}

// 使用中间件挂载
viewer.Mount(mux, authMiddleware)
```

## 🗄️ 数据库支持

| 数据库      | 单数据库 | 多数据库 | 功能 |
|------------|---------|---------|------|
| PostgreSQL | ✅      | ✅      | 完全支持，使用 pg_* 系统表 |
| MySQL      | ✅      | ✅      | 完全支持，使用 SHOW 命令 |
| SQLite     | ✅      | ❌      | 单文件连接 |

### 驱动要求

SQLView 不导入数据库驱动以避免强制依赖。请导入您需要的驱动：

```go
import (
    _ "github.com/lib/pq"                // PostgreSQL
    _ "github.com/go-sql-driver/mysql"   // MySQL
    _ "modernc.org/sqlite"               // SQLite（纯 Go，无 CGO）
    // 或
    // _ "github.com/mattn/go-sqlite3"   // SQLite（CGO，更快）
)
```

## 🎯 功能特性

### 表操作

**右键点击任意表**即可访问：

- 📊 **查看数据** - 显示表内容（默认前 100 行）
- 📐 **查看表结构** - 列名、类型、可空、默认值
- 🔑 **查看索引** - 索引名、类型、列、唯一性
- 📝 **查看 DDL** - 完整的 CREATE TABLE 和 CREATE INDEX 语句
- 📈 **查看统计信息** - 行数、表大小、索引大小
- 📋 **复制表名** - 复制到剪贴板

### SQL 查询编辑器

- ✏️ 支持语法高亮的多行 SQL 编辑器
- ⌨️ 键盘快捷键：`Ctrl/Cmd + Enter` 执行查询
- 🔒 只读模式：仅允许 SELECT 和 WITH 查询
- 💾 查询结果 CSV 导出
- 📊 格式化的结果表格

### 智能特性

- 🔍 **表搜索** - 实时表名过滤
- 🎯 **智能排序** - 自动按 id、updated_at、created_at 降序排序
- 🌐 **多数据库切换** - 浏览不同的数据库
- 📱 **响应式设计** - 支持桌面和移动端
- ⌨️ **键盘快捷键** - ESC 关闭弹窗/菜单

## 🔒 安全性

⚠️ **重要安全提示：**

1. SQLView 仅允许 `SELECT` 和 `WITH` 查询
2. 不支持 INSERT、UPDATE、DELETE 或 DDL 操作
3. **建议**：生产环境使用认证中间件
4. **建议**：仅限内部网络访问

```go
// 示例：基本认证中间件
func basicAuth(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
    user, pass, ok := r.BasicAuth()
    if !ok || user != "admin" || pass != "secret" {
        w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return r, false
    }
    return r, true
}

viewer.Mount(mux, basicAuth)
```

## 📖 API 参考

### 类型

```go
// New 创建一个自动检测数据库类型的 SQLView 实例
func New(db *sql.DB, basePath string) *SQLView

// NewWithDSN 使用显式配置创建 SQLView 实例
func NewWithDSN(db *sql.DB, basePath, dsn, driverName string) *SQLView

// Mount 将路由注册到 HTTP mux，支持可选的中间件
func (sv *SQLView) Mount(mux interface{
    HandleFunc(string, func(http.ResponseWriter, *http.Request))
}, preHandles ...PreHandle)

// Handler 返回一个独立的 HTTP handler
func (sv *SQLView) Handler() http.Handler

// PreHandle 是中间件函数类型
type PreHandle func(w http.ResponseWriter, r *http.Request) (*http.Request, bool)
```

### API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/connection-info` | GET | 数据库连接信息 |
| `/api/databases` | GET | 列出数据库/schema |
| `/api/tables?database=<db>` | GET | 列出表 |
| `/api/table-data?database=<db>&table=<name>` | GET | 表数据（100 行）|
| `/api/table-schema?database=<db>&table=<name>` | GET | 表结构 |
| `/api/table-indexes?database=<db>&table=<name>` | GET | 表索引 |
| `/api/table-ddl?database=<db>&table=<name>` | GET | CREATE TABLE DDL |
| `/api/table-stats?database=<db>&table=<name>` | GET | 表统计信息 |
| `/api/query` | POST | 执行 SELECT 查询 |

## 🛠️ 示例

查看 [examples](examples/) 目录获取完整示例：

- [PostgreSQL](examples/postgres/) - 基本 PostgreSQL 示例
- [MySQL](examples/mysql/) - 基本 MySQL 示例
- [SQLite](examples/sqlite/) - 内存 SQLite 示例
- [多数据库](examples/multi-db/) - 多数据库浏览
- [中间件](examples/middleware/) - 认证示例

## 🤝 贡献

欢迎贡献！请随时提交 Pull Request。

1. Fork 本仓库
2. 创建您的特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交您的更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启一个 Pull Request

## 📄 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 🙏 致谢

- 受简单、可嵌入数据库查看器需求的启发
- 使用原生 JavaScript 构建（无框架！）
- 使用 Go 的 `embed` 包实现零依赖部署

## 📮 联系方式

- GitHub: [@vito-go](https://github.com/vito-go)
- 项目: [github.com/vito-go/sqlview](https://github.com/vito-go/sqlview)

---

<div align="center">
由 Go 社区用 ❤️ 制作
</div>
