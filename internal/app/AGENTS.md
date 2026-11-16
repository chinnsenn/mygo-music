# app/
> L2 | 父级: ../../AGENTS.md

成员清单

app.go: app 状态根与生命周期，持有窗口、托盘、播放器、音源库与设置；启动时装配播放器（含 LyricConfig 与 LyricFallback）、窗口、菜单与退出钩子
artwork.go: 封面解码与 artwork/artCache，按 GPU/CPU 限额缓存，供播放器与界面共用
backdrop.go: Now Playing 背景的雾状渐变与低分辨率画布缓存，随音乐漂移
cache.go: 歌曲磁盘缓存 newSongCache、限额应用 applyCacheLimit 与用量测量
captions.go: 播客字幕的本地转写状态机，按播放位置增量写入磁盘，支持回退与续写
debug_shots.go: MM_SHOTS 驱动的界面走查截图，不触碰用户设置
download.go: 歌曲另存为的原生对话框与可下载判断，测试可替换
icons.go: 24 点网格上的 SVG 图标集（播放控制、导航、设置分类等）
menu.go: 按语言重建应用菜单与播放器菜单
notch_darwin.go: macOS 刘海与菜单栏几何计算、置顶窗口
notch_other.go: 非 macOS 的刘海几何空实现
plugin_install.go: 插件从 GitHub、文件夹或 zip 的安装与更新流程，需确认后才安装
request_headers.go: headerPlugins 收集启用插件的图片请求头，供非主线程取图
retro_render.go: 复古播放器（黑胶、磁带）的像素级预渲染位图
session.go: session.json 的播放现场保存与恢复，定时与退出时写入
settings.go: settings 结构与 settings.json 的默认值、载入、保存、外观应用；LyricFallback 与歌词目录 LyricConfig 位于此处，载入后归一化
sources.go: T/TC/Tf/N 翻译入口、音源能力表与 library/音源加载、失败提示
tray.go: macOS 菜单栏与 Windows/Linux 通知区图标，控制关窗后继续播放
ui_account.go: 账户行的菜单、登录对话框与 loadAccount
ui_album.go: 专辑页数据与加载、收藏切换与分页
ui_artist.go: 歌手页、艺人链接与专辑链接渲染
ui_browse.go: 音源自建页面（Home、专题、see-all）的列表化构建
ui_captions.go: Now Playing 的字幕面板与模型下载提示
ui_comments.go: 歌曲/节目评论的分页状态与评论页渲染
ui_cover.go: Now Playing 封面的倾斜、拖拽、甩动与切歌手势
ui_daily.go: Home 的每日主打卡片与封面排版
ui_kit.go: palette 配色、useAccent、通用布局与控件样式
ui_like.go: 收藏心的状态、逐帧询问与批量同步
ui_lyric_settings.go: 跨平台歌词补全的紧凑下级表单：无内层卡片，统一控件列，说明通过悬停提示呈现；关闭时隐藏并保留保存值
ui_main.go: 主窗口视图 view、侧边栏、页面路由打开/show 与快捷键装配
ui_notch.go: 刘海快捷播放器的展开、播放控制与动画
ui_nowplaying.go: Now Playing 的进出过渡（弹簧、淡入淡出）与整体层
ui_pages.go: 网格列数、Home 问候与首页/专题页组装
ui_player.go: 进度条 scrubState、拖动 seek、音量与烟雾动效
ui_playlistedit.go: 自建歌单的增删改与歌曲增删移、命名对话框
ui_podcast.go: 节目页数据、节目单分页与单集播放
ui_quiet.go: 安静段落的歌词淡出与热评浮现
ui_radio.go: 电台/无限播放的 station 命名与状态
ui_retro.go: 按 PlayerStyle 在 Now Playing 渲染复古播放器并承接手势
ui_search.go: 搜索类型标签、结果分页与搜索状态
ui_settings.go: 设置窗口视图与动作；settingsSections 的 Experiments 页含跨源歌词补全开关与歌词参数行，changed 统一保存并下发播放器
ui_skeleton.go: 加载骨架块与 shimmer 动效
ui_songmenu.go: 歌曲三点/右键菜单（复制、链接、歌单、下载等）
ui_source_settings.go: 音源设置控件（密码、布尔、文本）与防重复的异步保存
ui_stage.go: 舞台歌词可视化（Flow），按播放位置逐词弹入与发光
ui_stage_styles.go: 舞台样式注册表：Flow、Steps、Tilt 等 Folia 可视化
ui_suggest.go: 搜索联想防抖与 suggestState
ui_wall.go: 队列封面墙的拖拽、甩动、滚动与点击播放
updates.go: 更新插件的检查与偏好封装，测试可替换
websignin.go: 仅支持网页登录的插件的登录窗口与 Cookie 判定

account_test.go: 账户菜单焦点/键盘、My Page 跳转
artist_test.go: 歌手页入口、Albums 标签与返回路径
artwork_test.go: 动态 WebP 首帧、超大封面缩放与像素释放
backdrop_test.go: 背景漂移/低音随动与 MM_FRAMES 截图
captions_test.go: 字幕分段、断点续写与模型缺失路径
catalog_test.go: 搜索请求与 fakeSearch 基建
comments_test.go: 评论页加载、标签计数与热评
cover_test.go: 封面拖拽/甩动切歌
covered_test.go: Now Playing 覆盖时下层页面不构建、关闭后滚动位置保留
daily_test.go: 每日主打卡片打开与返回
edges_test.go: 长页滚动边缘渐显
fade_test.go: 封面加载淡入
frames_test.go: MM_FRAMES 下的界面外观截图
freshen_test.go: 重开歌单的后台刷新（issue #9）
helpers_test.go: TestMain、mainThread、fakeSource、newTestApp、posts/runPost 等测试基建
history_test.go: 鼠标侧键前进后退
home_test.go: 本地音源首页与音源自建 Home
like_test.go: 歌曲/单集收藏与设置页交互
lyric_settings_test.go: 跨源歌词开关及参数保存；关闭后子设置与搜索结果隐藏，重新开启后参数仍可编辑
notch_test.go: 刘海播放器首次展示与按钮保持
np_test.go: Now Playing 整体进出与队列点击播放
perf_test.go: 待机/播放帧预算
playlistedit_test.go: 歌单增删改、加歌与菜单
quiet_test.go: 安静段落热评轮换
radio_test.go: 电台播放与 station 命名
retro_test.go: 设置选择复古样式与甩动切歌
saveplaylist_test.go: 收藏他人歌单与失败回滚
scrub_test.go: 进度条拖动只在松手时 seek
session_test.go: 播放现场定时保存且不空转重绘
sidebar_test.go: 歌单分组折叠与 Enter 恢复导航
skeleton_test.go: 各加载视图骨架与明暗截图
songmenu_test.go: 歌曲菜单复制/链接与右键不播放
sources_test.go: 多音源切换、插件设置保存、登录与来源页行为
space_test.go: 打开 Now Playing 后 Space 暂停而非二次点击
stage_test.go: 舞台样式与火花菜单
ui_kit_test.go: 时长/计数的人性化格式
ui_source_settings_test.go: 插件设置读写同步、布尔开关幂等保存
updates_test.go: fakeUpdater 更新检查计数
volume_scroll_test.go: 滚轮调音量与钳制
wall_test.go: 队列封面墙布局与点击
websignin_test.go: Cookie 完成登录与 Cookie 头拼装

子目录：
testdata/: 测试图片素材（animated.webp）
tray/: 菜单栏/通知区图标（tray.png、tray-template.png）

法则: 成员完整·一行一文件·父级链接·技术词前置

[PROTOCOL]: 变更时更新此头部，然后检查 AGENTS.md
