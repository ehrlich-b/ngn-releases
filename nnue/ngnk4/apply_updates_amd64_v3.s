//go:build amd64 && amd64.v3

#include "go_asm.h"
#include "textflag.h"

// The K4 update rows are exact modular int16 arithmetic. VPADDW and VPSUBW
// therefore match the Go int16 oracle lane-for-lane, including overflow.
TEXT ·k4ApplyUpdates2AVX2(SB), NOSPLIT, $0-24
	MOVQ destination+0(FP), AX
	MOVQ add0+8(FP), BX
	MOVQ subtract1+16(FP), CX
	MOVL $const_k4LaneBlocks, R8
k4_apply_updates_2_block:
	VMOVDQU (AX), Y0
	VPADDW (BX), Y0, Y0
	VPSUBW (CX), Y0, Y0
	VMOVDQU Y0, (AX)
	ADDQ $32, AX
	ADDQ $32, BX
	ADDQ $32, CX
	DECL R8
	JNZ k4_apply_updates_2_block
	VZEROUPPER
	RET

TEXT ·k4ApplyUpdates3AVX2(SB), NOSPLIT, $0-32
	MOVQ destination+0(FP), AX
	MOVQ add0+8(FP), BX
	MOVQ subtract1+16(FP), CX
	MOVQ subtract2+24(FP), DX
	MOVL $const_k4LaneBlocks, R8
k4_apply_updates_3_block:
	VMOVDQU (AX), Y0
	VPADDW (BX), Y0, Y0
	VPSUBW (CX), Y0, Y0
	VPSUBW (DX), Y0, Y0
	VMOVDQU Y0, (AX)
	ADDQ $32, AX
	ADDQ $32, BX
	ADDQ $32, CX
	ADDQ $32, DX
	DECL R8
	JNZ k4_apply_updates_3_block
	VZEROUPPER
	RET

TEXT ·k4ApplyUpdates4AVX2(SB), NOSPLIT, $0-40
	MOVQ destination+0(FP), AX
	MOVQ add0+8(FP), BX
	MOVQ subtract1+16(FP), CX
	MOVQ add2+24(FP), DX
	MOVQ subtract3+32(FP), R8
	MOVL $const_k4LaneBlocks, R9
k4_apply_updates_4_block:
	VMOVDQU (AX), Y0
	VPADDW (BX), Y0, Y0
	VPSUBW (CX), Y0, Y0
	VPADDW (DX), Y0, Y0
	VPSUBW (R8), Y0, Y0
	VMOVDQU Y0, (AX)
	ADDQ $32, AX
	ADDQ $32, BX
	ADDQ $32, CX
	ADDQ $32, DX
	ADDQ $32, R8
	DECL R9
	JNZ k4_apply_updates_4_block
	VZEROUPPER
	RET
