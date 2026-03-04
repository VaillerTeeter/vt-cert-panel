/** 文件：app.js | 用途：证书管理面板的前端交互逻辑 | 关键函数：checkBootstrap, api, loadCertificates, setMessage, renderCertificates */

// DOM 元素缓存区 - 页面加载后立即获取核心元素的引用，避免重复查询 DOM

// setupCard 表示管理员初始化卡片（首次使用时展示） | 类型：HTMLElement | 作用域：全局
const setupCard = document.getElementById('setupCard');
// loginCard 表示登录卡片（已初始化但未登录时展示） | 类型：HTMLElement | 作用域：全局
const loginCard = document.getElementById('loginCard');
// panelCard 表示主控制面板卡片（登录成功后展示） | 类型：HTMLElement | 作用域：全局
const panelCard = document.getElementById('panelCard');
// certTable 表示证书列表表格的 tbody 元素（用于动态渲染证书行） | 类型：HTMLElement | 作用域：全局
const certTable = document.getElementById('certTable');
// message 表示全局消息显示区域（显示操作成功/失败提示） | 类型：HTMLElement | 作用域：全局
// message 表示全局消息显示区域（显示操作成功/失败提示） | 类型：HTMLElement | 作用域：全局
const message = document.getElementById('message');

// setMessage 用于在页面顶部显示操作提示消息 | 输入：text（消息文本），success（是否成功，默认 false） | 输出：无 | 副作用：修改 message 元素的文本内容和 CSS 类名
function setMessage(text, success = false) {
    // 操作 DOM 元素 #message：设置文本内容（为空时显示空字符串）
    message.textContent = text || '';
    // 操作 DOM 元素 #message：根据 success 参数设置 CSS 类名（成功时为 'msg ok'，失败时为 'msg'）
    message.className = success ? 'msg ok' : 'msg';
}

// api 用于统一封装所有后端 API 调用，处理 HTTP 请求与响应 | 输入：path（API 路径），options（fetch 选项对象，可选） | 输出：解析后的 JSON 数据对象 | 副作用：发起网络请求，失败时抛出异常
async function api(path, options = {}) {
    // 使用 async/await 或 Promise，等待 fetch 网络请求完成，超时无或浏览器默认超时 时处理
    // 调用后端 API {path}，请求类型根据 options.method 决定（默认 GET），预期响应：JSON 格式的数据对象或错误信息 | 失败处理：抛出包含错误信息的 Error 对象
    const resp = await fetch(path, {
        // 设置请求头，告诉后端发送的是 JSON 格式数据
        headers: { 'Content-Type': 'application/json' },
        // 启用 Cookie 凭证传递（用于 Session 认证）
        credentials: 'include',
        // 展开用户传入的其他选项（如 method、body 等）
        ...options,
    });

    // 尝试解析响应体为 JSON，解析失败时返回空对象（避免后续代码报错）
    const data = await resp.json().catch(() => ({}));

    // 检查 HTTP 状态码，如果不是 2xx 成功状态则抛出异常
    if (!resp.ok) {
        // 抛出包含后端错误信息的异常（优先使用 data.error，否则使用通用提示）
        throw new Error(data.error || '请求失败');
    }

    // 返回解析后的 JSON 数据对象
    return data;
}

// renderCertificates 用于渲染证书列表表格 | 输入：items（证书对象数组） | 输出：无 | 副作用：清空并重新填充 certTable 的内容，生成表格行元素
function renderCertificates(items) {
    // 操作 DOM 元素 #certTable：清空表格内容（删除所有旧的证书行）
    certTable.innerHTML = '';
    // 遍历证书数组，为每个证书对象生成一行表格
    for (const item of items) {
        // 创建新的表格行元素 <tr>
        const tr = document.createElement('tr');
        // 设置表格行的 HTML 内容（包含 6 列数据）
        tr.innerHTML = `
        <td>${item.id}</td>
        <td>${item.primaryDomain}</td>
        <td>${(item.domains || []).join('<br/>')}</td>
        <td>${item.expiresAt ? new Date(item.expiresAt).toLocaleString() : '-'}</td>
        <td>${item.status || '-'}</td>
        <td><a href="/api/certificates/${item.id}/download">下载ZIP</a></td>
        `;
        // 操作 DOM 元素 #certTable：将新创建的行追加到表格中
        certTable.appendChild(tr);
    }
}

// loadCertificates 用于从后端加载证书列表并渲染到页面 | 输入：无 | 输出：无（异步操作） | 副作用：调用 API 获取证书列表，然后调用 renderCertificates 更新 DOM
async function loadCertificates() {
    // 调用后端 API /api/certificates，请求类型 GET，预期响应：{ items: [ { id, primaryDomain, domains, expiresAt, status } ] } | 失败处理：抛出异常由调用方捕获
    const data = await api('/api/certificates');
    // 渲染证书列表（如果 data.items 为空则渲染空数组，避免报错）
    renderCertificates(data.items || []);
}

// checkBootstrap 用于检查系统初始化状态并切换 UI 界面 | 输入：无 | 输出：无（异步操作） | 副作用：调用 API 检查是否已初始化，根据结果显示/隐藏不同的卡片
async function checkBootstrap() {
    // 调用后端 API /api/bootstrap，请求类型 GET，预期响应：{ initialized: true/false } | 失败处理：抛出异常由调用方捕获
    const data = await api('/api/bootstrap', { method: 'GET' });
    // 检查系统是否已初始化管理员账号
    if (!data.initialized) {
        // 未初始化时显示管理员设置卡片
        // 操作 DOM 元素 #setupCard：移除 hidden 类名（显示元素）
        setupCard.classList.remove('hidden');
        // 操作 DOM 元素 #loginCard：添加 hidden 类名（隐藏元素）
        loginCard.classList.add('hidden');
        // 操作 DOM 元素 #panelCard：添加 hidden 类名（隐藏元素）
        panelCard.classList.add('hidden');
        return;
    }

    // 已初始化时隐藏设置卡片，显示登录卡片
    // 操作 DOM 元素 #setupCard：添加 hidden 类名（隐藏元素）
    setupCard.classList.add('hidden');
    // 操作 DOM 元素 #loginCard：移除 hidden 类名（显示元素）
    loginCard.classList.remove('hidden');
}

// 监听元素 #setupBtn 的事件 click，触发时 执行管理员初始化操作（创建首个管理员账号）
document.getElementById('setupBtn').addEventListener('click', async () => {
    try {
        // 调用后端 API /api/bootstrap/admin，请求类型 POST，预期响应：成功时返回空对象或 { message: "..." } | 失败处理：捕获异常并显示错误消息
        await api('/api/bootstrap/admin', {
            method: 'POST',
            // 发送 JSON 格式的请求体（包含用户名和密码）
            body: JSON.stringify({
                // 从输入框获取用户名并去除首尾空格
                username: document.getElementById('setupUsername').value.trim(),
                // 从输入框获取密码（不去除空格，保持原始值）
                password: document.getElementById('setupPassword').value,
            }),
        });

        // 初始化成功后显示提示消息（绿色成功样式）
        setMessage('管理员初始化成功，请登录。', true);
        // 重新检查初始化状态，切换到登录界面
        await checkBootstrap();
    } catch (e) {
        // 捕获异常并显示错误消息（红色错误样式）
        setMessage(e.message);
    }
});

// 监听元素 #loginBtn 的事件 click，触发时 执行用户登录操作（验证用户名和密码）
document.getElementById('loginBtn').addEventListener('click', async () => {
    try {
        // 调用后端 API /api/login，请求类型 POST，预期响应：成功时返回空对象并设置 Session Cookie | 失败处理：捕获异常并显示错误消息
        await api('/api/login', {
            method: 'POST',
            // 发送 JSON 格式的请求体（包含用户名和密码）
            body: JSON.stringify({
                // 从输入框获取用户名并去除首尾空格
                username: document.getElementById('loginUsername').value.trim(),
                // 从输入框获取密码（不去除空格，保持原始值）
                password: document.getElementById('loginPassword').value,
            }),
        });

        // 登录成功后隐藏登录卡片
        // 操作 DOM 元素 #loginCard：添加 hidden 类名（隐藏元素）
        loginCard.classList.add('hidden');
        // 操作 DOM 元素 #panelCard：移除 hidden 类名（显示主控制面板）
        panelCard.classList.remove('hidden');

        // 显示登录成功提示消息（绿色成功样式）
        setMessage('登录成功。', true);
        // 加载并显示证书列表
        await loadCertificates();
    } catch (e) {
        // 捕获异常并显示错误消息（红色错误样式）
        setMessage(e.message);
    }
});

// 监听元素 #logoutBtn 的事件 click，触发时 执行用户退出登录操作（销毁 Session）
document.getElementById('logoutBtn').addEventListener('click', async () => {
    try {
        // 调用后端 API /api/logout，请求类型 POST，预期响应：成功时返回空对象并清除 Session Cookie | 失败处理：忽略异常（即使失败也执行 UI 切换）
        await api('/api/logout', { method: 'POST' });
    } catch (_) {}

    // 无论 API 调用成功与否，都执行以下 UI 切换操作
    // 操作 DOM 元素 #panelCard：添加 hidden 类名（隐藏主控制面板）
    panelCard.classList.add('hidden');
    // 操作 DOM 元素 #loginCard：移除 hidden 类名（显示登录界面）
    loginCard.classList.remove('hidden');

    // 显示退出登录提示消息（绿色成功样式）
    setMessage('已退出登录。', true);
});

// 监听元素 #applyBtn 的事件 click，触发时 执行证书申请操作（向后端提交域名和邮箱信息）
document.getElementById('applyBtn').addEventListener('click', async () => {
    try {
        // 调用后端 API /api/certificates，请求类型 POST，预期响应：{ id, primaryDomain, domains, status, ... } | 失败处理：捕获异常并显示错误消息
        const data = await api('/api/certificates', {
            method: 'POST',
            // 发送 JSON 格式的请求体（包含域名列表和联系邮箱）
            body: JSON.stringify({
                // 从输入框获取域名字符串（多个域名用逗号或空格分隔）
                domains: document.getElementById('domainsInput').value,
                // 从输入框获取邮箱地址并去除首尾空格
                email: document.getElementById('emailInput').value.trim(),
            }),
        });

        // 证书申请成功后显示提示消息（绿色成功样式，包含主域名信息）
        setMessage(`证书申请成功：${data.primaryDomain}`, true);
        // 重新加载证书列表，显示新申请的证书
        await loadCertificates();
    } catch (e) {
        // 捕获异常并显示错误消息（红色错误样式）
        setMessage(e.message);
    }
});

// 页面加载时立即执行的初始化逻辑（使用立即执行的异步函数表达式 IIFE）
(async () => {
    // 检查系统初始化状态并切换 UI 界面（显示设置卡片或登录卡片）
    await checkBootstrap();
})();
