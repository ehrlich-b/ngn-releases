//go:build amd64 && amd64.v3

#include "textflag.h"

// The caller preserves semantic update order by invoking this kernel once for
// each row. Within a row, lanes are independent float32 additions.
TEXT ·addFeatureRowAVX2(SB), NOSPLIT, $0-16
	MOVQ accumulator+0(FP), AX
	MOVQ weights+8(FP), BX
	MOVL $64, CX
add_loop:
	VMOVUPS (AX), Y0
	VADDPS (BX), Y0, Y0
	VMOVUPS Y0, (AX)
	ADDQ $32, AX
	ADDQ $32, BX
	DECL CX
	JNZ add_loop
	VZEROUPPER
	RET

// Subtraction retains accumulator-weight operand order for every lane.
TEXT ·subFeatureRowAVX2(SB), NOSPLIT, $0-16
	MOVQ accumulator+0(FP), AX
	MOVQ weights+8(FP), BX
	MOVL $64, CX
sub_loop:
	VMOVUPS (AX), Y0
	VSUBPS (BX), Y0, Y0
	VMOVUPS Y0, (AX)
	ADDQ $32, AX
	ADDQ $32, BX
	DECL CX
	JNZ sub_loop
	VZEROUPPER
	RET
