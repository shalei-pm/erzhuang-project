import { type FormEvent, useEffect, useMemo, useState } from "react";
import { storeSpaceApi, type EzvizAccount, type LiveAddressResult } from "../api";
import { errorMessage } from "../domain/format";

type Props = {
  appVersion: string;
};

export function EzvizLiveDemo({ appVersion }: Props) {
  const [accounts, setAccounts] = useState<EzvizAccount[]>([]);
  const [accountId, setAccountId] = useState<number | "">("");
  const [accountName, setAccountName] = useState("华北");
  const [deviceSerial, setDeviceSerial] = useState("");
  const [channelNo, setChannelNo] = useState(1);
  const [code, setCode] = useState("");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<LiveAddressResult | null>(null);
  const [message, setMessage] = useState("");

  const selectedAccount = useMemo(() => accounts.find((account) => account.id === accountId), [accountId, accounts]);

  useEffect(() => {
    void storeSpaceApi
      .listEzvizAccounts()
      .then((items) => {
        setAccounts(items);
        const north = items.find((item) => item.accountName === "华北");
        if (north) {
          setAccountId(north.id);
          setAccountName(north.accountName);
        }
      })
      .catch((error) => setMessage(errorMessage(error, "萤石云账号加载失败。")));
  }, []);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setLoading(true);
    setMessage("");
    setResult(null);
    try {
      const liveAddress = await storeSpaceApi.getLiveAddress({
        ezvizAccountId: accountId,
        accountName: selectedAccount?.accountName || accountName,
        deviceSerial,
        channelNo,
        code,
      });
      setResult(liveAddress);
      setMessage("接口返回成功，正在尝试播放。");
    } catch (error) {
      setMessage(errorMessage(error, "获取播放地址失败。"));
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="app-shell demo-shell">
      <header className="page-header">
        <div>
          <p className="eyebrow">萤石云播放诊断</p>
          <h1>A 方案播放地址 Demo</h1>
        </div>
        <a className="demo-back-link" href={window.location.pathname}>
          返回系统
        </a>
      </header>

      <section className="demo-layout">
        <form className="demo-panel" onSubmit={handleSubmit}>
          <label>
            选择区域
            <select
              value={accountId}
              onChange={(event) => {
                const nextId = event.target.value ? Number(event.target.value) : "";
                setAccountId(nextId);
                const nextAccount = accounts.find((account) => account.id === nextId);
                if (nextAccount) setAccountName(nextAccount.accountName);
              }}
            >
              <option value="">按账号名称测试</option>
              {accounts.map((account) => (
                <option key={account.id} value={account.id}>
                  {account.accountName}
                </option>
              ))}
            </select>
          </label>

          <label>
            账号名称
            <input value={accountName} onChange={(event) => setAccountName(event.target.value)} placeholder="华北" />
          </label>

          <label>
            录像机设备编码
            <input value={deviceSerial} onChange={(event) => setDeviceSerial(event.target.value)} placeholder="请输入北方实验录像机编码" />
          </label>

          <label>
            通道号
            <input
              type="number"
              min={1}
              value={channelNo}
              onChange={(event) => setChannelNo(Math.max(1, Number(event.target.value) || 1))}
            />
          </label>

          <label>
            设备验证码
            <input value={code} onChange={(event) => setCode(event.target.value)} placeholder="设备加密开启时必填" />
          </label>

          <button className="primary-button" disabled={loading || !deviceSerial.trim()}>
            {loading ? "获取中" : "获取播放地址"}
          </button>

          {message ? <p className="demo-message">{message}</p> : null}
        </form>

        <section className="demo-player-panel" aria-label="播放验证">
          {result?.url ? (
            <>
              <video className="demo-video" src={result.url} controls playsInline autoPlay muted />
              <dl className="demo-result">
                <div>
                  <dt>协议</dt>
                  <dd>{result.protocol || "-"}</dd>
                </div>
                <div>
                  <dt>URL ID</dt>
                  <dd>{result.urlId || "-"}</dd>
                </div>
                <div>
                  <dt>过期时间</dt>
                  <dd>{result.expireTime || "-"}</dd>
                </div>
                <div>
                  <dt>播放地址</dt>
                  <dd>{safeURLSummary(result.url)}</dd>
                </div>
              </dl>
            </>
          ) : (
            <div className="demo-empty">输入北方实验录像机编码和通道号后，验证萤石播放地址接口是否可用。</div>
          )}
        </section>
      </section>

      <footer className="app-version" aria-label="当前版本">
        版本 {appVersion}
      </footer>
    </main>
  );
}

function safeURLSummary(value: string) {
  try {
    const url = new URL(value);
    return `${url.protocol}//${url.host}${url.pathname}`;
  } catch {
    return value.slice(0, 96);
  }
}
