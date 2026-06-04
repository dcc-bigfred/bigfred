import type { SvgIconComponent } from "@mui/icons-material";
import FunctionsIcon from "@mui/icons-material/Functions";
import LightbulbIcon from "@mui/icons-material/Lightbulb";
import SettingsIcon from "@mui/icons-material/Settings";
import VolumeUpIcon from "@mui/icons-material/VolumeUp";
import HornIcon from "@mui/icons-material/Campaign";
import LinkIcon from "@mui/icons-material/Link";
import DoorFrontIcon from "@mui/icons-material/DoorFront";
import CloudIcon from "@mui/icons-material/Cloud";
import SpeakerIcon from "@mui/icons-material/Speaker";
import SportsEsportsIcon from "@mui/icons-material/SportsEsports";
import WcIcon from "@mui/icons-material/Wc";
import CompressIcon from "@mui/icons-material/Compress";
import ConstructionIcon from "@mui/icons-material/Construction";
import AirIcon from "@mui/icons-material/Air";
import PanToolIcon from "@mui/icons-material/PanTool";
import WaterDropIcon from "@mui/icons-material/WaterDrop";
import VolumeOffIcon from "@mui/icons-material/VolumeOff";
import RadioIcon from "@mui/icons-material/Radio";
import SwapHorizIcon from "@mui/icons-material/SwapHoriz";
import BuildIcon from "@mui/icons-material/Build";
import TireRepairIcon from "@mui/icons-material/TireRepair";
import GrainIcon from "@mui/icons-material/Grain";
import RecordVoiceOverIcon from "@mui/icons-material/RecordVoiceOver";
import WifiIcon from "@mui/icons-material/Wifi";
import SignalCellularAltIcon from "@mui/icons-material/SignalCellularAlt";
import VolumeDownIcon from "@mui/icons-material/VolumeDown";
import Inventory2Icon from "@mui/icons-material/Inventory2";
import OpacityIcon from "@mui/icons-material/Opacity";
import ArrowUpwardIcon from "@mui/icons-material/ArrowUpward";
import ArrowDownwardIcon from "@mui/icons-material/ArrowDownward";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import ArrowForwardIcon from "@mui/icons-material/ArrowForward";
import LocalFireDepartmentIcon from "@mui/icons-material/LocalFireDepartment";
import WindowIcon from "@mui/icons-material/Window";
import WarningIcon from "@mui/icons-material/Warning";
import EmojiEmotionsIcon from "@mui/icons-material/EmojiEmotions";
import StairsIcon from "@mui/icons-material/Stairs";
import FlareIcon from "@mui/icons-material/Flare";
import TurnLeftIcon from "@mui/icons-material/TurnLeft";
import TurnRightIcon from "@mui/icons-material/TurnRight";

const ICON_MAP: Record<string, SvgIconComponent> = {
  unspecified: FunctionsIcon,
  light: LightbulbIcon,
  engine: SettingsIcon,
  sound: VolumeUpIcon,
  horn: HornIcon,
  coupler: LinkIcon,
  interior_light: LightbulbIcon,
  engine_room_light: LightbulbIcon,
  shunting_steps_light: LightbulbIcon,
  inspection_light: LightbulbIcon,
  cab_light: LightbulbIcon,
  headlight: LightbulbIcon,
  roof_headlight: LightbulbIcon,
  red_lights: FlareIcon,
  vestibule_lights: LightbulbIcon,
  destination_board_lights: LightbulbIcon,
  door: DoorFrontIcon,
  smoke: CloudIcon,
  speaker: SpeakerIcon,
  whistle: RecordVoiceOverIcon,
  toilet: WcIcon,
  compressor: CompressIcon,
  brake_sound: VolumeUpIcon,
  coal_shoveling: ConstructionIcon,
  fan: AirIcon,
  hand_brake: PanToolIcon,
  injector: WaterDropIcon,
  mute_sounds: VolumeOffIcon,
  radio_command: RadioIcon,
  shunting_mode: SwapHorizIcon,
  valve: BuildIcon,
  wheels: TireRepairIcon,
  wipers: GrainIcon,
  sander: GrainIcon,
  long_whistle: RecordVoiceOverIcon,
  short_whistle: RecordVoiceOverIcon,
  pantograph: LinkIcon,
  volume_up: VolumeUpIcon,
  volume_down: VolumeDownIcon,
  heavy_load: Inventory2Icon,
  wifi: WifiIcon,
  pc2_signal: SignalCellularAltIcon,
  coupling: LinkIcon,
  uncoupling: LinkIcon,
  oil_pump: OpacityIcon,
  brake_sound_mute: VolumeOffIcon,
  wheel_squeal: SportsEsportsIcon,
  bell: RecordVoiceOverIcon,
  coal_bunker: ConstructionIcon,
  watering: WaterDropIcon,
  crane_up: ArrowUpwardIcon,
  crane_down: ArrowDownwardIcon,
  crane_left: ArrowBackIcon,
  crane_right: ArrowForwardIcon,
  crane_hook: LinkIcon,
  sifa: WarningIcon,
  firebox: LocalFireDepartmentIcon,
  steam_release: CloudIcon,
  window: WindowIcon,
  buffer: LinkIcon,
  danger: WarningIcon,
  engineer_laugh: EmojiEmotionsIcon,
  stairs: StairsIcon,
  beacon_light: FlareIcon,
  side_lights: LightbulbIcon,
  turn_signal_left: TurnLeftIcon,
  turn_signal_right: TurnRightIcon,
};

export function FunctionIconVisual({
  icon,
  fontSize = "small",
}: {
  icon: string;
  fontSize?: "small" | "medium" | "large";
}) {
  const Cmp = ICON_MAP[icon] ?? FunctionsIcon;
  return <Cmp fontSize={fontSize} />;
}
