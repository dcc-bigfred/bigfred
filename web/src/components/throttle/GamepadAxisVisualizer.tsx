import { Box, Stack, Typography } from "@mui/material";

function axisPercent(value: number): number {
  return ((Math.max(-1, Math.min(1, value)) + 1) / 2) * 100;
}

interface AxisBarProps {
  label: string;
  value: number;
  idleMin?: number;
  idleMax?: number;
  emphasized?: boolean;
}

function AxisBar({ label, value, idleMin, idleMax, emphasized }: AxisBarProps) {
  const marker = axisPercent(value);
  const hasIdle =
    idleMin != null && idleMax != null && idleMin <= idleMax;
  const idleLeft = hasIdle ? axisPercent(idleMin) : 0;
  const idleWidth = hasIdle ? axisPercent(idleMax) - idleLeft : 0;

  return (
    <Stack spacing={0.25}>
      <Stack direction="row" justifyContent="space-between" alignItems="baseline">
        <Typography
          variant={emphasized ? "body2" : "caption"}
          fontWeight={emphasized ? 600 : 400}
        >
          {label}
        </Typography>
        <Typography variant="caption" color="text.secondary" fontFamily="monospace">
          {value.toFixed(3)}
        </Typography>
      </Stack>
      <Box
        sx={{
          position: "relative",
          height: emphasized ? 14 : 10,
          borderRadius: 1,
          bgcolor: "action.hover",
          border: 1,
          borderColor: emphasized ? "primary.main" : "divider",
        }}
      >
        {hasIdle && (
          <Box
            sx={{
              position: "absolute",
              top: 0,
              bottom: 0,
              left: `${idleLeft}%`,
              width: `${idleWidth}%`,
              bgcolor: "success.main",
              opacity: 0.35,
              borderRadius: 0.5,
            }}
          />
        )}
        <Box
          sx={{
            position: "absolute",
            top: "50%",
            left: `${marker}%`,
            width: emphasized ? 4 : 3,
            height: emphasized ? 18 : 14,
            bgcolor: "primary.main",
            borderRadius: 0.5,
            transform: "translate(-50%, -50%)",
            boxShadow: 1,
          }}
        />
        <Box
          sx={{
            position: "absolute",
            top: 0,
            bottom: 0,
            left: "50%",
            width: 1,
            bgcolor: "divider",
            transform: "translateX(-50%)",
          }}
        />
      </Box>
    </Stack>
  );
}

export interface GamepadAxisVisualizerProps {
  axes: number[];
  speedAxis: number;
  title?: string;
  idleMin?: number;
  idleMax?: number;
  liveLearnMin?: number | null;
  liveLearnMax?: number | null;
}

export default function GamepadAxisVisualizer({
  axes,
  speedAxis,
  title,
  idleMin,
  idleMax,
  liveLearnMin,
  liveLearnMax,
}: GamepadAxisVisualizerProps) {
  if (axes.length === 0) {
    return null;
  }

  const learning =
    liveLearnMin != null && liveLearnMax != null
      ? { min: liveLearnMin, max: liveLearnMax }
      : null;
  const idleZone =
    learning ??
    (idleMin != null && idleMax != null
      ? { min: idleMin, max: idleMax }
      : null);

  return (
    <Box>
      {title && (
        <Typography variant="subtitle2" gutterBottom>
          {title}
        </Typography>
      )}
      <Stack spacing={1}>
        {axes.map((value, index) => (
          <AxisBar
            key={index}
            label={`A${index}${index === speedAxis ? " *" : ""}`}
            value={value}
            emphasized={index === speedAxis}
            idleMin={index === speedAxis ? idleZone?.min : undefined}
            idleMax={index === speedAxis ? idleZone?.max : undefined}
          />
        ))}
      </Stack>
    </Box>
  );
}
