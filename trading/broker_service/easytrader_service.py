#!/usr/bin/env python3
"""
EasyTrader微服务 - 提供券商接口的REST API
支持华泰(ht)、银河(yh)、佣金宝(yjb)等券商
"""

import os
import logging
from datetime import datetime
from typing import Optional, Dict, List, Any
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException, status
from pydantic import BaseModel, Field
import uvicorn
import easytrader


# 配置日志
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


# 全局变量
trader = None
broker_type = "yh"
trader_connected = False


# 请求/响应模型
class LoginRequest(BaseModel):
    broker_type: str = Field(..., description="券商类型: ht, yh, yjb, xq等")
    username: Optional[str] = Field(None, description="用户名")
    password: Optional[str] = Field(None, description="密码")
    exe_path: Optional[str] = Field(None, description="券商客户端路径")


class BuyRequest(BaseModel):
    symbol: str = Field(..., description="股票代码，如 sh600000")
    price: float = Field(..., gt=0, description="买入价格")
    amount: int = Field(..., gt=0, description="买入数量")


class SellRequest(BaseModel):
    symbol: str = Field(..., description="股票代码，如 sh600000")
    price: float = Field(..., gt=0, description="卖出价格")
    amount: int = Field(..., gt=0, description="卖出数量")


class CancelRequest(BaseModel):
    order_id: str = Field(..., description="委托编号")


class ResponseModel(BaseModel):
    success: bool
    message: str
    data: Optional[Any] = None
    timestamp: str


def create_response(success: bool, message: str, data: Any = None) -> ResponseModel:
    """创建标准响应"""
    return ResponseModel(
        success=success,
        message=message,
        data=data,
        timestamp=datetime.now().isoformat()
    )


@asynccontextmanager
async def lifespan(app: FastAPI):
    """应用生命周期管理"""
    global trader, trader_connected
    # 启动时初始化
    logger.info("EasyTrader服务启动中...")
    broker_type_env = os.getenv("BROKER_TYPE", "yh")
    username = os.getenv("BROKER_USERNAME", "")
    password = os.getenv("BROKER_PASSWORD", "")
    
    if username and password:
        try:
            trader = easytrader.use(broker_type_env)
            if broker_type_env == "yh":
                trader.prepare(os.getenv("BROKER_EXE_PATH", ""))
                trader.login()
            else:
                trader.prepare(os.getenv("BROKER_EXE_PATH", ""))
                trader.login(username, password)
            trader_connected = True
            logger.info(f"成功登录券商: {broker_type_env}")
        except Exception as e:
            logger.warning(f"自动登录失败: {e}, 请手动调用login接口")
            trader_connected = False
    else:
        logger.info("未配置自动登录，请调用login接口")
    
    yield
    
    # 关闭时清理
    logger.info("EasyTrader服务关闭中...")
    if trader:
        try:
            trader.exit()
        except Exception as e:
            logger.warning(f"退出券商客户端失败: {e}")


# 创建FastAPI应用
app = FastAPI(
    title="EasyTrader微服务",
    description="提供券商接口REST API",
    version="1.0.0",
    lifespan=lifespan
)


@app.get("/health", response_model=ResponseModel)
async def health_check():
    """健康检查"""
    return create_response(
        success=True,
        message="服务正常",
        data={"connected": trader_connected, "broker_type": broker_type}
    )


@app.post("/login", response_model=ResponseModel)
async def login(req: LoginRequest):
    """登录券商"""
    global trader, trader_connected, broker_type
    
    try:
        trader = easytrader.use(req.broker_type)
        broker_type = req.broker_type
        
        # 根据券商类型调用不同的登录方法
        if req.broker_type == "yh":
            # 银河证券需要先prepare，然后直接登录
            exe_path = req.exe_path or os.getenv("BROKER_EXE_PATH", "")
            if exe_path:
                trader.prepare(exe_path)
            else:
                trader.prepare()
            trader.login()
        else:
            # 其他券商
            exe_path = req.exe_path or os.getenv("BROKER_EXE_PATH", "")
            if exe_path:
                trader.prepare(exe_path)
            else:
                trader.prepare()
            trader.login(req.username, req.password)
        
        trader_connected = True
        logger.info(f"成功登录券商: {req.broker_type}")
        
        return create_response(
            success=True,
            message=f"成功登录券商: {req.broker_type}",
            data={"broker_type": req.broker_type}
        )
    except Exception as e:
        trader_connected = False
        logger.error(f"登录失败: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"登录失败: {str(e)}"
        )


@app.get("/logout", response_model=ResponseModel)
async def logout():
    """退出登录"""
    global trader, trader_connected
    
    if not trader:
        return create_response(success=False, message="未登录")
    
    try:
        trader.exit()
        trader_connected = False
        trader = None
        return create_response(success=True, message="已退出登录")
    except Exception as e:
        logger.error(f"退出登录失败: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"退出登录失败: {str(e)}"
        )


@app.post("/buy", response_model=ResponseModel)
async def buy(req: BuyRequest):
    """买入股票"""
    if not trader or not trader_connected:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="未登录券商"
        )
    
    try:
        # easytrader买入返回委托编号
        result = trader.buy(req.symbol, price=req.price, amount=req.amount)
        logger.info(f"买入请求: {req.symbol}, 价格: {req.price}, 数量: {req.amount}, 结果: {result}")
        
        return create_response(
            success=True,
            message="买入请求已提交",
            data={
                "symbol": req.symbol,
                "price": req.price,
                "amount": req.amount,
                "order_id": result
            }
        )
    except Exception as e:
        logger.error(f"买入失败: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"买入失败: {str(e)}"
        )


@app.post("/sell", response_model=ResponseModel)
async def sell(req: SellRequest):
    """卖出股票"""
    if not trader or not trader_connected:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="未登录券商"
        )
    
    try:
        result = trader.sell(req.symbol, price=req.price, amount=req.amount)
        logger.info(f"卖出请求: {req.symbol}, 价格: {req.price}, 数量: {req.amount}, 结果: {result}")
        
        return create_response(
            success=True,
            message="卖出请求已提交",
            data={
                "symbol": req.symbol,
                "price": req.price,
                "amount": req.amount,
                "order_id": result
            }
        )
    except Exception as e:
        logger.error(f"卖出失败: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"卖出失败: {str(e)}"
        )


@app.post("/cancel", response_model=ResponseModel)
async def cancel(req: CancelRequest):
    """撤销委托"""
    if not trader or not trader_connected:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="未登录券商"
        )
    
    try:
        result = trader.cancel_entrust(req.order_id)
        logger.info(f"撤销委托: {req.order_id}, 结果: {result}")
        
        return create_response(
            success=True,
            message="撤销委托请求已提交",
            data={"order_id": req.order_id, "result": result}
        )
    except Exception as e:
        logger.error(f"撤销委托失败: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"撤销委托失败: {str(e)}"
        )


@app.get("/portfolio", response_model=ResponseModel)
async def get_portfolio():
    """获取持仓信息"""
    if not trader or not trader_connected:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="未登录券商"
        )
    
    try:
        positions = trader.position
        logger.info(f"获取持仓: {len(positions)} 只股票")
        
        return create_response(
            success=True,
            message="获取持仓成功",
            data=positions
        )
    except Exception as e:
        logger.error(f"获取持仓失败: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"获取持仓失败: {str(e)}"
        )


@app.get("/balance", response_model=ResponseModel)
async def get_balance():
    """获取账户余额"""
    if not trader or not trader_connected:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="未登录券商"
        )
    
    try:
        balance = trader.balance
        logger.info(f"获取余额: {balance}")
        
        return create_response(
            success=True,
            message="获取余额成功",
            data=balance
        )
    except Exception as e:
        logger.error(f"获取余额失败: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"获取余额失败: {str(e)}"
        )


@app.get("/orders", response_model=ResponseModel)
async def get_orders():
    """获取当日委托"""
    if not trader or not trader_connected:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="未登录券商"
        )
    
    try:
        orders = trader.today_entrusts
        logger.info(f"获取当日委托: {len(orders)} 条")
        
        return create_response(
            success=True,
            message="获取当日委托成功",
            data=orders
        )
    except Exception as e:
        logger.error(f"获取当日委托失败: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"获取当日委托失败: {str(e)}"
        )


@app.get("/today_trades", response_model=ResponseModel)
async def get_today_trades():
    """获取当日成交"""
    if not trader or not trader_connected:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="未登录券商"
        )
    
    try:
        trades = trader.today_trades
        logger.info(f"获取当日成交: {len(trades)} 条")
        
        return create_response(
            success=True,
            message="获取当日成交成功",
            data=trades
        )
    except Exception as e:
        logger.error(f"获取当日成交失败: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"获取当日成交失败: {str(e)}"
        )


if __name__ == "__main__":
    port = int(os.getenv("EASYTRADER_PORT", "8888"))
    uvicorn.run(
        "easytrader_service:app",
        host="0.0.0.0",
        port=port,
        reload=False,
        log_level="info"
    )
