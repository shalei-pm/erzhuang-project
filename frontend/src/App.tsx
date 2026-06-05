const checks = [
  "Vite + React + TypeScript 已接入",
  "生产构建输出到 frontend/dist",
  "后续可由 nginx /erzhuang/ 或 Go 静态文件服务承载",
];

function App() {
  return (
    <main className="app-shell">
      <section className="intro">
        <p className="eyebrow">erzhuang-project</p>
        <h1>前端工程基础已就位</h1>
        <p className="summary">
          当前页面用于验证本地开发服务器和生产构建流程，暂不接入真实业务和服务器配置。
        </p>
      </section>

      <section className="status-panel" aria-label="前端工程状态">
        {checks.map((item) => (
          <div className="status-row" key={item}>
            <span className="status-dot" aria-hidden="true" />
            <span>{item}</span>
          </div>
        ))}
      </section>
    </main>
  );
}

export default App;
