import asyncio
import websockets


async def echo(ws):
    async for msg in ws:
        await ws.send("echo:" + msg)


async def main():
    async with websockets.serve(echo, "127.0.0.1", 8765):
        await asyncio.Future()


asyncio.run(main())
