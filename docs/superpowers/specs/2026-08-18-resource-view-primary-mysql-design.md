# 资源查看收敛至二壮主库设计

日期：2026-08-18

## 背景

原 `db_groupbuy` 的资源查看数据已按安全要求同步至二壮测试和线上 MySQL `db_pm_erzhuang`。测试库只读核查确认四张表均存在，其中 `tb_crm_iot_area_device_relation` 有 5,632 条关系记录。

现有 3.0 启动时额外读取 `BUSINESS_MYSQL_DSN`，建立第二条 MySQL 连接。该设计曾受跨环境网络隔离影响，导致资源查看被降级。既然数据已同步进二壮主库，应移除第二数据源。

真实关系数据还包含 `pad`、`business_tv`、`live_tv`、`help_button`、`bt_gateway` 等非摄像头用途。3.0 当前产品范围是空间到摄像头/NVR/工控机映射，不能把这些合法非摄像头关系误报为缺失摄像头。

## 目标

- 资源查看复用二壮主 MySQL 连接，查询仍严格只读。
- 删除独立业务库 DSN 的代码和部署依赖。
- 只将摄像头设备关系用于空间摄像头绑定和摄像头完整性异常项。
- 对“摄像头用途但设备不存在”的关系保留异常提示。
- 保持 API 路径、登录授权、普通查看用户的监控门店范围、H5 Monitor 播放链路和页面结构不变。

## 数据规则

```text
空间：tb_crm_consulting_room
设备：tb_crm_iot_device
关系：tb_crm_iot_area_device_relation

摄像头关系 = 设备 category='camera'
             或设备不存在且 function_type 以 'camera' 结尾
```

- 有效空间摄像头绑定只接受同时存在空间和 `category='camera'` 设备的关系。
- 非摄像头用途关系不进入摄像头绑定数、未绑定摄像头数或“缺失摄像头”异常。
- 设备已缺失但 `function_type` 为摄像头用途的关系保留为 `missing_camera` 异常。
- 使用 `(device_id, area_id, function_type)` 作为原始关系唯一键；空间与摄像头的显示绑定按 `(device_id, area_id)` 去重，避免同一摄像头因多用途重复显示。

## 实施边界

涉及：`cmd/server/main.go`、`cmd/server/main_test.go`、`internal/resourceview/mysql_repository.go`、对应 Go 测试、README 和发布文档。

不涉及：数据库 DDL、同步任务、任何写 SQL、H5 Monitor、播放器、前端信息架构、旧 2.x 回滚能力。

## 验收标准

1. 代码不再读取或引用 `BUSINESS_MYSQL_DSN` 与 `K8S_SECRET_BUSINESS_MYSQL_DSN`。
2. 配置主 MySQL 后，资源查看服务随主库初始化；主库未配置时服务保持未启用，不引入第二连接。
3. 资源 repository 只含读取 SQL，且四张同步表查询不产生写操作。
4. `security_camera`、`cart_camera` 等摄像头关系可形成空间绑定；PAD、电视、网关等关系不产生 `missing_camera`。
5. 摄像头用途的孤儿设备关系仍产生 `missing_camera` 异常。
6. 本地 Go 测试、前端测试和生产构建通过；发布测试环境后用真实数据验收列表、详情和异常项。
