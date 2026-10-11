"""Load helper modules without registering a live NoneBot plugin during unit tests.
Production plugin startup still imports the real __init__ and driver normally.
"""
from pathlib import Path
import sys
from types import ModuleType

package = ModuleType("nonebot_plugin_nowork")
package.__path__ = [str(Path(__file__).resolve().parents[1] / "nonebot_plugin_nowork")]
sys.modules[package.__name__] = package
